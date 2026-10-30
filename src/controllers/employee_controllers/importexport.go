package employeecontrollers

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	employeedb "ismila/erp/zkbio/database/employee_db"
	milastructs "ismila/erp/zkbio/mila_structs"
	employeestructs "ismila/erp/zkbio/mila_structs/employee_structs"

	"github.com/gin-gonic/gin"
)

// csvHeader is the export column order, and the header an import is expected to
// carry. Export writes exactly these so a file can be exported, edited in a
// spreadsheet and imported straight back.
var csvHeader = []string{"employeeNumber", "name", "cardNumber", "privilege", "source", "isActive", "lastSeenAt"}

// maxImportErrors caps the errors echoed back, so one badly-mangled file cannot
// produce an unbounded response body.
const maxImportErrors = 50

// maxImportBytes bounds an uploaded CSV. Generous for any real headcount while
// keeping a hostile upload from exhausting the Pi's memory.
const maxImportBytes = 8 << 20 // 8 MB

// ExportEmployees streams the registry as CSV. Honours the same filters as the
// list endpoint, so "export what I am looking at" works.
func ExportEmployees(c *gin.Context) {
	f := employeedb.EmployeeFilter{
		Search:   c.Query("q"),
		Source:   c.Query("source"),
		DeviceID: c.Query("deviceId"),
		Active:   c.Query("active"),
	}

	employees, resp := employeedb.SelectAllEmployees(f)
	if resp != nil && !resp.IsSuccess {
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	filename := "employees-" + time.Now().Format("2006-01-02") + ".csv"
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Header("Content-Type", "text/csv; charset=utf-8")

	w := csv.NewWriter(c.Writer)
	if err := w.Write(csvHeader); err != nil {
		return // client went away mid-stream; nothing useful to report
	}
	for _, e := range employees {
		lastSeen := ""
		if e.LastSeenAt.Valid {
			lastSeen = e.LastSeenAt.Time.Format(time.RFC3339)
		}
		rec := []string{
			e.EmployeeNumber,
			e.Name,
			e.CardNumber,
			strconv.Itoa(e.Privilege),
			e.Source,
			strconv.FormatBool(e.IsActive),
			lastSeen,
		}
		if err := w.Write(rec); err != nil {
			return
		}
	}
	w.Flush()
}

// ImportEmployees upserts employees from a CSV upload.
//
// Accepts either a multipart form (field "file", what a browser sends) or a raw
// CSV request body (what curl sends). Rows upsert on employeeNumber, so
// re-importing a corrected file fixes names rather than duplicating people.
//
// An imported name is authoritative: the row is stamped source='import' and a
// later device sync will not overwrite it.
func ImportEmployees(c *gin.Context) {
	reader, err := importReader(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, milastructs.Fail(err.Error()))
		return
	}

	result, err := importCSV(reader)
	if err != nil {
		c.JSON(http.StatusBadRequest, milastructs.Fail(err.Error()))
		return
	}
	c.JSON(http.StatusOK, milastructs.Ok(
		fmt.Sprintf("imported %d rows: %d created, %d updated, %d skipped, %d failed",
			result.Rows, result.Created, result.Updated, result.Skipped, result.Failed),
		result))
}

// importReader picks the CSV source: a multipart "file" field, else the raw body.
func importReader(c *gin.Context) (io.Reader, error) {
	if file, err := c.FormFile("file"); err == nil {
		f, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("cannot read uploaded file: %w", err)
		}
		// Closed when the request ends; gin keeps the body alive until then.
		c.Request.Body = f
		return io.LimitReader(f, maxImportBytes), nil
	}
	if c.Request.Body == nil {
		return nil, errors.New("no CSV supplied — POST a multipart form field 'file' or a raw text/csv body")
	}
	return io.LimitReader(c.Request.Body, maxImportBytes), nil
}

// importCSV parses and applies the rows.
func importCSV(r io.Reader) (*employeestructs.ImportResult, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	// Rows may legitimately have fewer trailing columns than the header when a
	// spreadsheet drops empty cells.
	cr.FieldsPerRecord = -1

	header, err := cr.Read()
	if err != nil {
		return nil, errors.New("could not read the CSV header row: " + err.Error())
	}
	cols, err := mapColumns(header)
	if err != nil {
		return nil, err
	}

	out := &employeestructs.ImportResult{}
	for line := 2; ; line++ { // line 1 was the header
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			out.Failed++
			addError(out, fmt.Sprintf("line %d: %v", line, err))
			continue
		}
		out.Rows++

		number := strings.TrimSpace(field(rec, cols["employeenumber"]))
		if number == "" {
			out.Skipped++
			addError(out, fmt.Sprintf("line %d: employeeNumber is empty", line))
			continue
		}

		e := &employeestructs.Employee{
			EmployeeNumber: number,
			Name:           strings.TrimSpace(field(rec, cols["name"])),
			CardNumber:     strings.TrimSpace(field(rec, cols["cardnumber"])),
			IsActive:       true,
		}
		if v := strings.TrimSpace(field(rec, cols["privilege"])); v != "" {
			e.Privilege, _ = strconv.Atoi(v)
		}
		if v := strings.TrimSpace(field(rec, cols["isactive"])); v != "" {
			if b, err := strconv.ParseBool(v); err == nil {
				e.IsActive = b
			}
		}

		switch outcome, resp := employeedb.UpsertFromOperator(e, employeestructs.SourceImport); outcome {
		case employeedb.Created:
			out.Created++
		case employeedb.Updated:
			out.Updated++
		default:
			out.Failed++
			addError(out, fmt.Sprintf("line %d (%s): %s", line, number, resp.Message))
		}
	}
	return out, nil
}

// mapColumns resolves header names to indexes, case- and separator-insensitively
// so "employeeNumber", "employee_number" and "Employee Number" all work — an
// operator round-tripping through Excel should not have to match our casing.
func mapColumns(header []string) (map[string]int, error) {
	cols := map[string]int{}
	for i, h := range header {
		key := normalizeHeader(h)
		// Accept a few friendly aliases for the two columns that matter.
		switch key {
		case "employeeno", "employeeid", "pin", "usernumber", "userid":
			key = "employeenumber"
		case "fullname", "employeename", "username":
			key = "name"
		case "card", "cardno":
			key = "cardnumber"
		}
		if _, seen := cols[key]; !seen {
			cols[key] = i
		}
	}
	if _, ok := cols["employeenumber"]; !ok {
		return nil, errors.New("CSV must have an 'employeeNumber' column (aliases: pin, employeeId, userId)")
	}
	return cols, nil
}

// normalizeHeader lowercases and strips separators and a UTF-8 BOM. Excel writes
// a BOM on the first cell, which would otherwise make the first column
// unrecognisable.
func normalizeHeader(h string) string {
	h = strings.TrimPrefix(h, "\uFEFF")
	h = strings.ToLower(strings.TrimSpace(h))
	h = strings.NewReplacer("_", "", "-", "", " ", "").Replace(h)
	return h
}

// field returns a column by index, tolerating a short row.
func field(rec []string, idx int) string {
	if idx < 0 || idx >= len(rec) {
		return ""
	}
	return rec[idx]
}

func addError(out *employeestructs.ImportResult, msg string) {
	if len(out.Errors) < maxImportErrors {
		out.Errors = append(out.Errors, msg)
		return
	}
	out.Truncate = true
}

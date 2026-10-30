package employeedb

import (
	"database/sql"
	"strings"

	gendb "ismila/erp/zkbio/database/gendb"
	milastructs "ismila/erp/zkbio/mila_structs"
	employeestructs "ismila/erp/zkbio/mila_structs/employee_structs"
)

// employeeColumns is the shared SELECT list.
const employeeColumns = `
	id, employee_number, name, card_number, privilege, device_user_id,
	source, last_device_id, is_active, last_seen_at, created_at, updated_at`

func scanEmployee(rows *sql.Rows) (*employeestructs.Employee, error) {
	var e employeestructs.Employee
	err := rows.Scan(
		&e.ID, &e.EmployeeNumber, &e.Name, &e.CardNumber, &e.Privilege, &e.DeviceUserID,
		&e.Source, &e.LastDeviceID, &e.IsActive, &e.LastSeenAt, &e.CreatedAt, &e.UpdatedAt,
	)
	return &e, err
}

// EmployeeFilter narrows a list query. Zero value = everything.
type EmployeeFilter struct {
	Search   string // matches employee_number or name
	Source   string // pull | push | import | manual
	DeviceID string // last seen on this device
	Active   string // "true" | "false" | "" for both
}

// where builds the shared WHERE clause so list, count and export always agree.
func (f EmployeeFilter) where() (string, []any) {
	var conds []string
	var args []any
	if s := strings.TrimSpace(f.Search); s != "" {
		conds = append(conds, `(employee_number LIKE ? OR name LIKE ?)`)
		like := "%" + s + "%"
		args = append(args, like, like)
	}
	if f.Source != "" {
		conds = append(conds, `source = ?`)
		args = append(args, f.Source)
	}
	if f.DeviceID != "" {
		conds = append(conds, `last_device_id = ?`)
		args = append(args, f.DeviceID)
	}
	switch f.Active {
	case "true":
		conds = append(conds, `is_active = TRUE`)
	case "false":
		conds = append(conds, `is_active = FALSE`)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// ListEmployees returns one page plus the total row count for that filter.
func ListEmployees(f EmployeeFilter, page, pageSize int) ([]*employeestructs.Employee, int64, *milastructs.AnswerResponse) {
	whereSQL, args := f.where()

	var total int64
	if err := gendb.SelectRow(`SELECT COUNT(*) FROM employees`+whereSQL, args...).Scan(&total); err != nil {
		r := milastructs.Err("db count failed: " + err.Error())
		return nil, 0, &r
	}

	q := `SELECT ` + employeeColumns + ` FROM employees` + whereSQL +
		` ORDER BY employee_number ASC LIMIT ? OFFSET ?`
	pageArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)

	var out []*employeestructs.Employee
	resp := gendb.SelectGeneral(func(rows *sql.Rows) error {
		for rows.Next() {
			e, err := scanEmployee(rows)
			if err != nil {
				return err
			}
			out = append(out, e)
		}
		return nil
	}, q, pageArgs...)
	return out, total, resp
}

// SelectAllEmployees returns every row matching the filter, ordered, for CSV
// export. Unpaginated on purpose — an export must be the whole set — which is
// safe because the registry is bounded by headcount, not by punch volume.
func SelectAllEmployees(f EmployeeFilter) ([]*employeestructs.Employee, *milastructs.AnswerResponse) {
	whereSQL, args := f.where()
	q := `SELECT ` + employeeColumns + ` FROM employees` + whereSQL + ` ORDER BY employee_number ASC`

	var out []*employeestructs.Employee
	resp := gendb.SelectGeneral(func(rows *sql.Rows) error {
		for rows.Next() {
			e, err := scanEmployee(rows)
			if err != nil {
				return err
			}
			out = append(out, e)
		}
		return nil
	}, q, args...)
	return out, resp
}

// SelectEmployeeByID returns one employee, or (nil, nil) if not found.
func SelectEmployeeByID(id string) (*employeestructs.Employee, *milastructs.AnswerResponse) {
	return selectOne(`SELECT `+employeeColumns+` FROM employees WHERE id = ?`, id)
}

// SelectEmployeeByNumber resolves by the device PIN / HR employee number.
func SelectEmployeeByNumber(number string) (*employeestructs.Employee, *milastructs.AnswerResponse) {
	return selectOne(`SELECT `+employeeColumns+` FROM employees WHERE employee_number = ?`, number)
}

func selectOne(q string, args ...any) (*employeestructs.Employee, *milastructs.AnswerResponse) {
	var out *employeestructs.Employee
	resp := gendb.SelectGeneral(func(rows *sql.Rows) error {
		if rows.Next() {
			e, err := scanEmployee(rows)
			if err != nil {
				return err
			}
			out = e
		}
		return nil
	}, q, args...)
	return out, resp
}

// NamesByNumbers resolves a set of employee numbers to names in one query, so a
// page of punches can be decorated with names without an N+1. Returns an empty
// map when numbers is empty, so callers need no special case.
func NamesByNumbers(numbers []string) (map[string]string, *milastructs.AnswerResponse) {
	out := map[string]string{}
	if len(numbers) == 0 {
		return out, nil
	}
	// Placeholders, never interpolation — these values come from devices.
	ph := strings.TrimSuffix(strings.Repeat("?,", len(numbers)), ",")
	args := make([]any, 0, len(numbers))
	for _, n := range numbers {
		args = append(args, n)
	}
	resp := gendb.SelectGeneral(func(rows *sql.Rows) error {
		for rows.Next() {
			var num, name string
			if err := rows.Scan(&num, &name); err != nil {
				return err
			}
			out[num] = name
		}
		return nil
	}, `SELECT employee_number, name FROM employees WHERE employee_number IN (`+ph+`)`, args...)
	return out, resp
}

// CountEmployees returns the registry size (dashboard).
func CountEmployees() (int64, *milastructs.AnswerResponse) {
	var n int64
	if err := gendb.SelectRow(`SELECT COUNT(*) FROM employees`).Scan(&n); err != nil {
		r := milastructs.Err("db count failed: " + err.Error())
		return 0, &r
	}
	return n, nil
}

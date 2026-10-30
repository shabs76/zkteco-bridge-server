package employeedb

import (
	"strings"

	gendb "ismila/erp/zkbio/database/gendb"
	milastructs "ismila/erp/zkbio/mila_structs"
	employeestructs "ismila/erp/zkbio/mila_structs/employee_structs"
)

// UpdateEmployee applies a partial update. Editing the name by hand stamps the
// row 'manual', which makes that name stick against later device syncs — the
// same precedence an imported row gets.
func UpdateEmployee(id string, r *employeestructs.UpdateEmployeeRequest) (int64, *milastructs.AnswerResponse) {
	var sets []string
	var args []any

	if r.Name != nil {
		sets = append(sets, "name = ?", "source = 'manual'")
		args = append(args, strings.TrimSpace(*r.Name))
	}
	if r.CardNumber != nil {
		sets = append(sets, "card_number = ?")
		args = append(args, strings.TrimSpace(*r.CardNumber))
	}
	if r.Privilege != nil {
		sets = append(sets, "privilege = ?")
		args = append(args, *r.Privilege)
	}
	if r.IsActive != nil {
		sets = append(sets, "is_active = ?")
		args = append(args, *r.IsActive)
	}
	if len(sets) == 0 {
		resp := milastructs.Fail("no updatable fields supplied")
		return 0, &resp
	}
	args = append(args, id)
	return gendb.UpdateGeneral(`UPDATE employees SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
}

// DeleteEmployee removes one registry row. Punches are deliberately untouched:
// they carry the employee_number themselves and are the record that must
// survive — deleting a name must never delete attendance.
func DeleteEmployee(id string) (int64, *milastructs.AnswerResponse) {
	return gendb.UpdateGeneral(`DELETE FROM employees WHERE id = ?`, id)
}

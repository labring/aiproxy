//go:build enterprise

package feishu

import (
	"strings"

	"github.com/labring/aiproxy/core/enterprise/models"
	"github.com/labring/aiproxy/core/model"
)

// DepartmentPath holds the hierarchical path of a department
type DepartmentPath struct {
	Level1ID   string `json:"level1_id"`
	Level1Name string `json:"level1_name"`
	Level2ID   string `json:"level2_id"`
	Level2Name string `json:"level2_name"`
	Level3ID   string `json:"level3_id"`
	Level3Name string `json:"level3_name"`
	FullPath   string `json:"full_path"`
}

// GetDepartmentPath builds the full hierarchical path for a department
func GetDepartmentPath(departmentID string) *DepartmentPath {
	if departmentID == "" || departmentID == "0" {
		return &DepartmentPath{}
	}

	// Build path by traversing up the parent chain
	var path []models.FeishuDepartment
	currentID := departmentID

	// Max depth of 10 to prevent infinite loops
	for i := 0; i < 10 && currentID != "" && currentID != "0"; i++ {
		var dept models.FeishuDepartment
		// Prefer the record with a name when duplicates exist
		// (e.g. one with department_id=od-xxx and one with the canonical ID)
		if err := model.DB.Where("department_id = ? OR open_department_id = ?", currentID, currentID).
			Order("CASE WHEN name != '' THEN 0 ELSE 1 END, updated_at DESC").
			First(&dept).Error; err != nil {
			break
		}

		path = append([]models.FeishuDepartment{dept}, path...)
		currentID = dept.ParentID
	}

	result := &DepartmentPath{}

	// Assign levels (from root to leaf)
	var names []string
	for i, dept := range path {
		name := dept.Name
		if name == "" {
			name = dept.DepartmentID // Fallback to ID if name is empty
		}
		names = append(names, name)

		switch i {
		case 0:
			result.Level1ID = dept.DepartmentID
			result.Level1Name = name
		case 1:
			result.Level2ID = dept.DepartmentID
			result.Level2Name = name
		case 2:
			result.Level3ID = dept.DepartmentID
			result.Level3Name = name
		}
	}

	result.FullPath = strings.Join(names, " > ")

	return result
}

// GetDepartmentTree returns a hierarchical tree of all departments
func GetDepartmentTree() ([]*models.FeishuDepartment, error) {
	var departments []models.FeishuDepartment

	if err := model.DB.Where("status = 1").
		Order("parent_id, `order`").
		Find(&departments).Error; err != nil {
		return nil, err
	}

	// Build tree structure
	deptMap := make(map[string]*models.FeishuDepartment)
	var roots []*models.FeishuDepartment

	for i := range departments {
		dept := &departments[i]
		deptMap[dept.DepartmentID] = dept
	}

	for _, dept := range deptMap {
		if dept.ParentID == "0" || dept.ParentID == "" {
			roots = append(roots, dept)
		}
	}

	return roots, nil
}

// GetLevel1Departments returns all top-level departments, deduplicated.
// When the same logical department has multiple records (e.g. one with custom department_id
// and one with open_department_id), prefer the record with a name.
func GetLevel1Departments() ([]*models.FeishuDepartment, error) {
	var departments []*models.FeishuDepartment

	if err := model.DB.Where("status = 1 AND (parent_id = '0' OR parent_id = '')").
		Order("CASE WHEN name != '' THEN 0 ELSE 1 END, `order`, name").
		Find(&departments).Error; err != nil {
		return nil, err
	}

	return deduplicateDepartments(departments), nil
}

// GetLevel2Departments returns all second-level departments under a given parent, deduplicated.
func GetLevel2Departments(level1ID string) ([]*models.FeishuDepartment, error) {
	if level1ID == "" {
		return nil, nil
	}

	// Resolve all ID forms for the parent department so we can match parent_id in any format
	parentIDs := GetAllDepartmentIDForms(level1ID)

	var departments []*models.FeishuDepartment

	if err := model.DB.Where("status = 1 AND parent_id IN ?", parentIDs).
		Order("CASE WHEN name != '' THEN 0 ELSE 1 END, `order`, name").
		Find(&departments).Error; err != nil {
		return nil, err
	}

	return deduplicateDepartments(departments), nil
}

// GetAllDepartmentIDForms returns all known ID forms (department_id and open_department_id)
// for a given department identifier. This handles the dual-ID system where the same department
// can be referenced by its custom ID or its od-* ID.
func GetAllDepartmentIDForms(deptID string) []string {
	idSet := map[string]struct{}{deptID: {}}

	var depts []models.FeishuDepartment
	model.DB.Where("department_id = ? OR open_department_id = ?", deptID, deptID).Find(&depts)

	for _, d := range depts {
		if d.DepartmentID != "" {
			idSet[d.DepartmentID] = struct{}{}
		}

		if d.OpenDepartmentID != "" {
			idSet[d.OpenDepartmentID] = struct{}{}
		}
	}

	result := make([]string, 0, len(idSet))
	for id := range idSet {
		result = append(result, id)
	}

	return result
}

// deduplicateDepartments removes duplicate department entries that represent
// the same logical department (sharing the same open_department_id).
// Prefers the record with a non-empty name.
func deduplicateDepartments(departments []*models.FeishuDepartment) []*models.FeishuDepartment {
	seen := make(map[string]*models.FeishuDepartment)
	var result []*models.FeishuDepartment

	for _, dept := range departments {
		key := dept.OpenDepartmentID
		if key == "" {
			key = dept.DepartmentID
		}

		existing, ok := seen[key]
		if !ok {
			seen[key] = dept
			result = append(result, dept)

			continue
		}

		// Replace if the new one has a name but the existing one doesn't
		if existing.Name == "" && dept.Name != "" {
			*existing = *dept
		}
	}

	return result
}

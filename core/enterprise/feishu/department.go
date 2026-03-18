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
		if err := model.DB.Where("department_id = ? OR open_department_id = ?", currentID, currentID).
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

// GetLevel1Departments returns all top-level departments
func GetLevel1Departments() ([]*models.FeishuDepartment, error) {
	var departments []*models.FeishuDepartment

	if err := model.DB.Where("status = 1 AND (parent_id = '0' OR parent_id = '')").
		Order("`order`, name").
		Find(&departments).Error; err != nil {
		return nil, err
	}

	return departments, nil
}

// GetLevel2Departments returns all second-level departments under a given parent
func GetLevel2Departments(level1ID string) ([]*models.FeishuDepartment, error) {
	if level1ID == "" {
		return nil, nil
	}

	var departments []*models.FeishuDepartment

	if err := model.DB.Where("status = 1 AND parent_id = ?", level1ID).
		Order("`order`, name").
		Find(&departments).Error; err != nil {
		return nil, err
	}

	return departments, nil
}

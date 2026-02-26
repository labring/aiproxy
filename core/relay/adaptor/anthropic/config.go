package anthropic

type Config struct {
	DisableContextManagement            bool     `json:"disable_context_management"`
	SupportedContextManagementEditsType []string `json:"supported_context_management_edits_type"`
	RemoveToolsExamples                 bool     `json:"remove_tools_examples"`
}

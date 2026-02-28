package notion

import (
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/sonic"
)

// ConvertToMarkdown converts Notion API response to Markdown
func ConvertToMarkdown(response any) (string, error) {
	if response == nil {
		return "", nil
	}

	// Try to determine response type
	jsonData, err := sonic.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	var baseObj map[string]any
	if err := sonic.Unmarshal(jsonData, &baseObj); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	objectType, ok := baseObj["object"].(string)
	if !ok {
		return fmt.Sprintf("```json\n%s\n```", string(jsonData)), nil
	}

	switch objectType {
	case "page":
		var page PageResponse
		if err := sonic.Unmarshal(jsonData, &page); err != nil {
			return "", err
		}

		return convertPageToMarkdown(&page), nil
	case "database":
		var database DatabaseResponse
		if err := sonic.Unmarshal(jsonData, &database); err != nil {
			return "", err
		}

		return convertDatabaseToMarkdown(&database), nil
	case "block":
		var block BlockResponse
		if err := sonic.Unmarshal(jsonData, &block); err != nil {
			return "", err
		}

		return convertBlockToMarkdown(&block), nil
	case "list":
		var list ListResponse
		if err := sonic.Unmarshal(jsonData, &list); err != nil {
			return "", err
		}

		return convertListToMarkdown(&list), nil
	default:
		return fmt.Sprintf("```json\n%s\n```", string(jsonData)), nil
	}
}

// convertPageToMarkdown converts a Notion page to Markdown
func convertPageToMarkdown(page *PageResponse) string {
	var markdown strings.Builder

	// Extract title
	title := extractPageTitle(page)
	if title != "" {
		fmt.Fprintf(&markdown, "# %s\n\n", title)
	}

	// Display page properties as a Markdown table
	markdown.WriteString(convertPropertiesToMarkdown(page.Properties))

	// Include additional information
	markdown.WriteString(
		"\n\n> This page contains child blocks. You can retrieve them using `retrieveBlockChildren`.\n",
	)
	fmt.Fprintf(&markdown, "> Block ID: `%s`\n", page.ID)

	// Add link to view the page in Notion
	if page.URL != "" {
		fmt.Fprintf(&markdown, "\n[View in Notion](%s)\n", page.URL)
	}

	return markdown.String()
}

// convertDatabaseToMarkdown converts a Notion database to Markdown
func convertDatabaseToMarkdown(database *DatabaseResponse) string {
	var markdown strings.Builder

	// Extract database title
	title := extractRichText(database.Title)
	if title != "" {
		fmt.Fprintf(&markdown, "# %s (Database)\n\n", title)
	}

	// Add description if available
	description := extractRichText(database.Description)
	if description != "" {
		markdown.WriteString(description + "\n\n")
	}

	// Display database property schema
	if len(database.Properties) > 0 {
		markdown.WriteString("## Properties\n\n")
		markdown.WriteString("| Property Name | Type | Details |\n")
		markdown.WriteString("|------------|------|------|\n")

		for key, prop := range database.Properties {
			propName := prop.Name
			if propName == "" {
				propName = key
			}

			propType := prop.Type
			if propType == "" {
				propType = "unknown"
			}

			// Additional information based on property type
			details := getPropertyDetails(prop)

			fmt.Fprintf(&markdown, "| %s | %s | %s |\n",
				escapeTableCell(propName),
				propType,
				escapeTableCell(details))
		}

		markdown.WriteString("\n")
	}

	// Add link to view the database in Notion
	if database.URL != "" {
		fmt.Fprintf(&markdown, "\n[View in Notion](%s)\n", database.URL)
	}

	return markdown.String()
}

// convertBlockToMarkdown converts Notion API block response to Markdown
func convertBlockToMarkdown(block *BlockResponse) string {
	if block == nil {
		return ""
	}

	return renderBlock(block)
}

// convertListToMarkdown converts list response to Markdown
func convertListToMarkdown(list *ListResponse) string {
	if list == nil || len(list.Results) == 0 {
		return "```\nNo results\n```"
	}

	var markdown strings.Builder

	// Determine the type of results
	firstResult := list.Results[0]
	resultType := getObjectType(firstResult)

	// Add header based on type
	switch resultType {
	case "page":
		markdown.WriteString("# Search Results (Pages)\n\n")
	case "database":
		markdown.WriteString("# Search Results (Databases)\n\n")
	case "block":
		markdown.WriteString("# Block Contents\n\n")
	default:
		markdown.WriteString("# Results List\n\n")
	}

	// Process each result
	for _, item := range list.Results {
		itemType := getObjectType(item)

		switch itemType {
		case "page":
			if resultType == "page" {
				// Simple display for search results
				if page := convertToPageResponse(item); page != nil {
					title := extractPageTitle(page)
					if title == "" {
						title = "Untitled"
					}

					url := page.URL
					if url == "" {
						url = "#"
					}

					fmt.Fprintf(&markdown, "## [%s](%s)\n\n", title, url)
					fmt.Fprintf(&markdown, "ID: `%s`\n\n", page.ID)
					markdown.WriteString("---\n\n")
				}
			} else {
				// Full conversion
				if page := convertToPageResponse(item); page != nil {
					markdown.WriteString(convertPageToMarkdown(page))
					markdown.WriteString("\n\n---\n\n")
				}
			}

		case "database":
			if resultType == "database" {
				// Simple display
				if db := convertToDatabaseResponse(item); db != nil {
					dbTitle := extractRichText(db.Title)
					if dbTitle == "" {
						dbTitle = "Untitled Database"
					}

					url := db.URL
					if url == "" {
						url = "#"
					}

					fmt.Fprintf(&markdown, "## [%s](%s)\n\n", dbTitle, url)
					fmt.Fprintf(&markdown, "ID: `%s`\n\n", db.ID)
					markdown.WriteString("---\n\n")
				}
			} else {
				// Full conversion
				if db := convertToDatabaseResponse(item); db != nil {
					markdown.WriteString(convertDatabaseToMarkdown(db))
					markdown.WriteString("\n\n---\n\n")
				}
			}

		case "block":
			if block := convertToBlockResponse(item); block != nil {
				markdown.WriteString(renderBlock(block))
				markdown.WriteString("\n\n")
			}

		default:
			jsonData, _ := sonic.MarshalIndent(item, "", "  ")
			fmt.Fprintf(&markdown, "```json\n%s\n```\n\n", string(jsonData))
		}
	}

	// Include pagination info if available
	if list.HasMore {
		markdown.WriteString(
			"\n> More results available. Use `start_cursor` parameter with the next request.\n",
		)

		if list.NextCursor != nil {
			fmt.Fprintf(&markdown, "> Next cursor: `%s`\n", *list.NextCursor)
		}
	}

	return markdown.String()
}

// extractPageTitle extracts page title from properties
func extractPageTitle(page *PageResponse) string {
	if page == nil || page.Properties == nil {
		return ""
	}

	for _, prop := range page.Properties {
		if prop.Type == "title" && len(prop.Title) > 0 {
			return extractRichText(prop.Title)
		}
	}

	return ""
}

// convertPropertiesToMarkdown converts page properties to Markdown
func convertPropertiesToMarkdown(properties map[string]PageProperty) string {
	if len(properties) == 0 {
		return ""
	}

	var markdown strings.Builder
	markdown.WriteString("## Properties\n\n")
	markdown.WriteString("| Property | Value |\n")
	markdown.WriteString("|------------|----|\n")

	for key, prop := range properties {
		propValue := getPropertyValue(prop)

		fmt.Fprintf(&markdown, "| %s | %s |\n",
			escapeTableCell(key),
			escapeTableCell(propValue))
	}

	return markdown.String()
}

// extractRichText extracts plain text from a Notion rich text array
func extractRichText(richTextArray []RichTextItemResponse) string {
	if len(richTextArray) == 0 {
		return ""
	}

	var result strings.Builder
	for _, item := range richTextArray {
		text := item.PlainText
		if text == "" && item.Text != nil {
			text = item.Text.Content
		}

		// Process annotations
		if item.Annotations != nil {
			if item.Annotations.Code {
				text = fmt.Sprintf("`%s`", text)
			}

			if item.Annotations.Bold {
				text = fmt.Sprintf("**%s**", text)
			}

			if item.Annotations.Italic {
				text = fmt.Sprintf("*%s*", text)
			}

			if item.Annotations.Strikethrough {
				text = fmt.Sprintf("~~%s~~", text)
			}
		}

		// Process links
		if item.Href != nil {
			text = fmt.Sprintf("[%s](%s)", text, *item.Href)
		}

		result.WriteString(text)
	}

	return result.String()
}

// renderBlock converts a block to Markdown
//
//nolint:gocyclo
func renderBlock(block *BlockResponse) string {
	if block == nil {
		return ""
	}

	switch block.Type {
	case BlockTypeParagraph:
		if block.Paragraph != nil {
			return renderParagraph(block.Paragraph)
		}
	case BlockTypeHeading1:
		if block.Heading1 != nil {
			return "# " + extractRichText(block.Heading1.RichText)
		}
	case BlockTypeHeading2:
		if block.Heading2 != nil {
			return "## " + extractRichText(block.Heading2.RichText)
		}
	case BlockTypeHeading3:
		if block.Heading3 != nil {
			return "### " + extractRichText(block.Heading3.RichText)
		}
	case BlockTypeBulletedListItem:
		if block.BulletedListItem != nil {
			return "- " + extractRichText(block.BulletedListItem.RichText)
		}
	case BlockTypeNumberedListItem:
		if block.NumberedListItem != nil {
			return "1. " + extractRichText(block.NumberedListItem.RichText)
		}
	case BlockTypeToDo:
		if block.ToDo != nil {
			checked := " "
			if block.ToDo.Checked {
				checked = "x"
			}

			return fmt.Sprintf("- [%s] %s", checked, extractRichText(block.ToDo.RichText))
		}
	case BlockTypeToggle:
		if block.Toggle != nil {
			return fmt.Sprintf(
				"<details>\n<summary>%s</summary>\n\n*Additional API request is needed to display child blocks*\n\n</details>",
				extractRichText(block.Toggle.RichText),
			)
		}
	case BlockTypeChildPage:
		if block.ChildPage != nil {
			return "📄 **Child Page**: " + block.ChildPage.Title
		}
	case BlockTypeImage:
		if block.Image != nil {
			imageURL := ""
			if block.Image.External != nil {
				imageURL = block.Image.External.URL
			} else if block.Image.File != nil {
				imageURL = block.Image.File.URL
			}

			imageCaption := extractRichText(block.Image.Caption)
			if imageCaption == "" {
				imageCaption = "image"
			}

			return fmt.Sprintf("![%s](%s)", imageCaption, imageURL)
		}
	case BlockTypeDivider:
		return "---"
	case BlockTypeQuote:
		if block.Quote != nil {
			return "> " + extractRichText(block.Quote.RichText)
		}
	case BlockTypeCode:
		if block.Code != nil {
			language := block.Code.Language
			if language == "" {
				language = "plaintext"
			}

			codeContent := extractRichText(block.Code.RichText)

			return fmt.Sprintf("```%s\n%s\n```", language, codeContent)
		}
	case BlockTypeCallout:
		if block.Callout != nil {
			icon := ""
			if block.Callout.Icon != nil && block.Callout.Icon.Emoji != "" {
				icon = block.Callout.Icon.Emoji + " "
			}

			text := extractRichText(block.Callout.RichText)

			return fmt.Sprintf("> %s%s", icon, text)
		}
	case BlockTypeBookmark:
		if block.Bookmark != nil {
			caption := extractRichText(block.Bookmark.Caption)
			if caption == "" {
				caption = block.Bookmark.URL
			}

			return fmt.Sprintf("[%s](%s)", caption, block.Bookmark.URL)
		}
	case BlockTypeChildDatabase:
		return fmt.Sprintf("📊 **Embedded Database**: `%s`", block.ID)
	default:
		return fmt.Sprintf("*Unsupported block type: %s*", block.Type)
	}

	return ""
}

// renderParagraph renders a paragraph block
func renderParagraph(paragraph *ParagraphBlock) string {
	if paragraph == nil || len(paragraph.RichText) == 0 {
		return ""
	}

	return extractRichText(paragraph.RichText)
}

// escapeTableCell escapes characters that need special handling in Markdown table cells
func escapeTableCell(text string) string {
	if text == "" {
		return ""
	}

	text = strings.ReplaceAll(text, "|", "\\|")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "+", "\\+")

	return text
}

// Helper functions for type conversion and property handling

func getObjectType(obj any) string {
	if objMap, ok := obj.(map[string]any); ok {
		if objectType, ok := objMap["object"].(string); ok {
			return objectType
		}
	}

	return "unknown"
}

func convertToPageResponse(obj any) *PageResponse {
	jsonData, err := sonic.Marshal(obj)
	if err != nil {
		return nil
	}

	var page PageResponse
	if err := sonic.Unmarshal(jsonData, &page); err != nil {
		return nil
	}

	return &page
}

func convertToDatabaseResponse(obj any) *DatabaseResponse {
	jsonData, err := sonic.Marshal(obj)
	if err != nil {
		return nil
	}

	var database DatabaseResponse
	if err := sonic.Unmarshal(jsonData, &database); err != nil {
		return nil
	}

	return &database
}

func convertToBlockResponse(obj any) *BlockResponse {
	jsonData, err := sonic.Marshal(obj)
	if err != nil {
		return nil
	}

	var block BlockResponse
	if err := sonic.Unmarshal(jsonData, &block); err != nil {
		return nil
	}

	return &block
}

func getPropertyDetails(prop DatabasePropertyConfig) string {
	switch prop.Type {
	case "select", "multi_select":
		if prop.Select != nil {
			var options []string
			for _, option := range prop.Select.Options {
				options = append(options, option.Name)
			}

			return "Options: " + strings.Join(options, ", ")
		}

		if prop.MultiSelect != nil {
			var options []string
			for _, option := range prop.MultiSelect.Options {
				options = append(options, option.Name)
			}

			return "Options: " + strings.Join(options, ", ")
		}
	case "relation":
		if prop.Relation != nil {
			return "Related DB: " + prop.Relation.DatabaseID
		}
	case "formula":
		if prop.Formula != nil {
			return "Formula: " + prop.Formula.Expression
		}
	case "rollup":
		if prop.Rollup != nil {
			return "Rollup: " + prop.Rollup.Function
		}
	case "number":
		if prop.Number != nil {
			return "Format: " + prop.Number.Format
		}
	case "status":
		if prop.Status != nil {
			var options []string
			for _, option := range prop.Status.Options {
				options = append(options, option.Name)
			}

			return "Options: " + strings.Join(options, ", ")
		}
	}

	// Default descriptions for common types
	switch prop.Type {
	case "created_by", "last_edited_by":
		return "User reference"
	case "created_time", "last_edited_time":
		return "Timestamp"
	case "date":
		return "Date or date range"
	case "email":
		return "Email address"
	case "files":
		return "File attachments"
	case "people":
		return "People reference"
	case "phone_number":
		return "Phone number"
	case "rich_text":
		return "Formatted text"
	case "title":
		return "Database title"
	case "url":
		return "URL link"
	case "checkbox":
		return "Boolean value"
	default:
		return ""
	}
}

//nolint:gocyclo
func getPropertyValue(prop PageProperty) string {
	switch prop.Type {
	case "title":
		return extractRichText(prop.Title)
	case "rich_text":
		return extractRichText(prop.RichText)
	case "number":
		if prop.Number != nil {
			return fmt.Sprintf("%.2f", *prop.Number)
		}
	case "select":
		if prop.Select != nil {
			return prop.Select.Name
		}
	case "multi_select":
		var names []string
		for _, item := range prop.MultiSelect {
			names = append(names, item.Name)
		}

		return strings.Join(names, ", ")
	case "date":
		if prop.Date != nil {
			result := prop.Date.Start
			if prop.Date.End != nil {
				result += " → " + *prop.Date.End
			}

			return result
		}
	case "people":
		var names []string
		for _, person := range prop.People {
			if person.ID != "" {
				names = append(names, person.ID)
			}
		}

		return strings.Join(names, ", ")
	case "files":
		var fileNames []string
		for _, file := range prop.Files {
			url := "#"
			if file.External != nil {
				url = file.External.URL
			} else if file.File != nil {
				url = file.File.URL
			}

			fileNames = append(fileNames, fmt.Sprintf("[%s](%s)", file.Name, url))
		}

		return strings.Join(fileNames, ", ")
	case "checkbox":
		if prop.Checkbox {
			return "✓"
		}
		return "✗"
	case "url":
		if prop.URL != nil {
			return *prop.URL
		}
	case "email":
		if prop.Email != nil {
			return *prop.Email
		}
	case "phone_number":
		if prop.PhoneNumber != nil {
			return *prop.PhoneNumber
		}
	case "formula":
		if prop.Formula != nil {
			if prop.Formula.String != nil {
				return *prop.Formula.String
			}

			if prop.Formula.Number != nil {
				return fmt.Sprintf("%.2f", *prop.Formula.Number)
			}

			if prop.Formula.Boolean != nil {
				if *prop.Formula.Boolean {
					return "true"
				}
				return "false"
			}
		}
	case "status":
		if prop.Status != nil {
			return prop.Status.Name
		}
	case "relation":
		var ids []string
		for _, relation := range prop.Relation {
			ids = append(ids, fmt.Sprintf("`%s`", relation.ID))
		}

		return strings.Join(ids, ", ")
	case "rollup":
		if prop.Rollup != nil {
			if prop.Rollup.Number != nil {
				return fmt.Sprintf("%.2f", *prop.Rollup.Number)
			}

			if prop.Rollup.String != nil {
				return *prop.Rollup.String
			}

			if prop.Rollup.Date != nil {
				return prop.Rollup.Date.Start
			}

			if prop.Rollup.Array != nil {
				jsonData, _ := sonic.Marshal(prop.Rollup.Array)
				return string(jsonData)
			}
		}
	case "created_by":
		if prop.CreatedBy != nil {
			return prop.CreatedBy.ID
		}
	case "created_time":
		if prop.CreatedTime != nil {
			return prop.CreatedTime.Format(time.RFC3339)
		}
	case "last_edited_by":
		if prop.LastEditedBy != nil {
			return prop.LastEditedBy.ID
		}
	case "last_edited_time":
		if prop.LastEditedTime != nil {
			return prop.LastEditedTime.Format(time.RFC3339)
		}
	}

	return "(Unsupported property type)"
}

package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
)

// newOptionValue is the sentinel option value for a "+ New" entry in a
// select-with-new prompt (see selectWithNew).
const newOptionValue = "\x00new"

func requireNonEmpty(s string) error {
	if s == "" {
		return fmt.Errorf("required")
	}
	return nil
}

// runForm runs a single-group huh form over fields.
func runForm(fields ...huh.Field) error {
	return huh.NewForm(huh.NewGroup(fields...)).Run()
}

// promptText prompts for required free text.
func promptText(title string) (string, error) {
	var value string
	if err := runForm(huh.NewInput().Title(title).Value(&value).Validate(requireNonEmpty)); err != nil {
		return "", err
	}
	return value, nil
}

// promptOptionalText prompts for free text with no non-empty validation,
// trimming whitespace. An empty result is a valid answer ("skip").
func promptOptionalText(title string) (string, error) {
	var value string
	if err := runForm(huh.NewInput().Title(title).Value(&value)); err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

// promptConfirm asks a yes/no question, defaulting to true.
func promptConfirm(title string) (bool, error) {
	confirmed := true
	if err := runForm(huh.NewConfirm().Title(title).Value(&confirmed)); err != nil {
		return false, err
	}
	return confirmed, nil
}

// stringOptions turns plain strings into identically-labeled huh options.
func stringOptions(items []string) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(items))
	for _, s := range items {
		options = append(options, huh.NewOption(s, s))
	}
	return options
}

// selectOneDefault prompts to pick one of options, preselecting def.
func selectOneDefault(title string, options []huh.Option[string], def string) (string, error) {
	value := def
	if err := runForm(huh.NewSelect[string]().Title(title).Options(options...).Value(&value)); err != nil {
		return "", err
	}
	return value, nil
}

// selectOne prompts to pick one of options, with no preselection.
func selectOne(title string, options []huh.Option[string]) (string, error) {
	return selectOneDefault(title, options, "")
}

// selectWithNew offers options plus a "+ New" entry that falls through to a
// free-text prompt at inputTitle. With no options, it prompts for free text
// directly.
func selectWithNew(selectTitle, inputTitle string, options []huh.Option[string]) (string, error) {
	if len(options) == 0 {
		return promptText(inputTitle)
	}
	options = append(options, huh.NewOption("+ New", newOptionValue))
	selected, err := selectOne(selectTitle, options)
	if err != nil {
		return "", err
	}
	if selected != newOptionValue {
		return selected, nil
	}
	return promptText(inputTitle)
}

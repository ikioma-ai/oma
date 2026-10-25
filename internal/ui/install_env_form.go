package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/ikioma-ai/oma/internal/catalog"
)

type InstallEnvSubmitMsg struct {
	Values map[string]string
}

type InstallEnvBackMsg struct{}

type InstallEnvForm struct {
	form   Form
	inputs []catalog.EnvVar
}

func NewInstallEnvForm(env []catalog.EnvVar) InstallEnvForm {
	inputs := envInputs(env)

	items := make([]FormItem, 0, len(inputs))
	for _, variable := range inputs {
		field := NewTextField(envPlaceholder(variable))
		if variable.Default != "" {
			field.SetValue(variable.Default)
		}
		if variable.Secret {
			field.SetEchoPassword()
		}

		label := variable.Name
		if variable.Prompt != "" {
			label = variable.Prompt + " (" + variable.Name + ")"
		}

		items = append(items, FormItem{
			Label:    label,
			Field:    field,
			Required: variable.Required,
		})
	}

	m := InstallEnvForm{
		form:   NewForm("Install", items...),
		inputs: inputs,
	}

	m.form.OnSubmit(func(f *Form) tea.Cmd {
		values := make(map[string]string, len(inputs))
		for i, variable := range inputs {
			if value := f.TextField(i).Value(); value != "" {
				values[variable.Name] = value
			}
		}
		return func() tea.Msg { return InstallEnvSubmitMsg{Values: values} }
	})

	m.form.OnCancel(func(_ *Form) tea.Cmd {
		return func() tea.Msg { return InstallEnvBackMsg{} }
	})

	return m
}

func (m InstallEnvForm) HasInputs() bool {
	return len(m.inputs) > 0
}

func (m InstallEnvForm) Init() tea.Cmd {
	return m.form.Init()
}

func (m InstallEnvForm) Update(msg tea.Msg) (InstallEnvForm, tea.Cmd) {
	var cmd tea.Cmd
	m.form, cmd = m.form.Update(msg)
	return m, cmd
}

func (m InstallEnvForm) View() string {
	return m.form.View()
}

// Helpers

func envInputs(env []catalog.EnvVar) []catalog.EnvVar {
	var inputs []catalog.EnvVar

	for _, variable := range env {
		if variable.Value != "" || variable.From != "" || variable.Generate != "" {
			continue
		}
		inputs = append(inputs, variable)
	}

	return inputs
}

func envPlaceholder(variable catalog.EnvVar) string {
	if variable.Secret {
		return "secret"
	}
	if variable.Required {
		return "required"
	}
	return "optional"
}

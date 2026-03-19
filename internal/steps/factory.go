package steps

import (
	koptan "github.com/felukka/koptan/api/v1alpha"
)

func Factory(step koptan.Step) Builder {
	switch step.Type {
	case "SonarQube":
		return NewSonarQube(step.Name, step.Params)
	}

	if step.Custom != nil {
		return NewCustom(step.Name, step.Custom)
	}

	return nil
}

// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schutils

import (
	"testing"

	_ "github.com/allons-y/openapi-analysis/internal/antest"
	spec "github.com/allons-y/openapi-spec"
	"github.com/go-openapi/testify/v2/assert"
)

func TestFlattenSchema_Save(t *testing.T) {
	t.Parallel()

	sp := &spec.Swagger{}
	Save(sp, "theName", spec.StringProperty())
	assert.NotNil(t, sp.Components)
	assert.Contains(t, sp.Components.Schemas, "theName")

	saveNilSchema := func() {
		Save(sp, "ThisNilSchema", nil)
	}
	assert.NotPanics(t, saveNilSchema)
}

func TestFlattenSchema_Clone(t *testing.T) {
	sch := spec.RefSchema("#/components/schemas/x")
	assert.Equal(t, sch, Clone(sch))
}

// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"fmt"
	"reflect"

	spec "github.com/allons-y/openapi-spec"
)

// Mixin modifies the primary OpenAPI spec by adding the paths and
// components from the mixin specs. Components (schemas, parameters, responses,
// and security schemes) from the mixins are also carried over. Operation id
// collisions are avoided by appending "Mixin<N>" but only if needed.
//
// The following parts of primary are subject to merge, filling empty details:
//   - Info
//   - ExternalDocs
//
// Consider calling FixEmptyResponseDescriptions() on the modified primary
// if you read them from storage and they are valid to start with.
//
// Entries in "paths" and "components" (schemas, parameters, responses, securitySchemes)
// are added to the primary in the order of the given mixins. If the entry
// already exists in primary it is skipped with a warning message.
//
// The count of skipped entries (from collisions) is returned so any
// deviation from the number expected can flag a warning in your build
// scripts. Carefully review the collisions before accepting them;
// consider renaming things if possible.
//
// No key normalization takes place (paths, type defs, etc).
// Ensure they are canonical if your downstream tools do key normalization
// of any form.
func Mixin(primary *spec.Swagger, mixins ...*spec.Swagger) []string {
	skipped := make([]string, 0, len(mixins))
	opIDs := getOpIDs(primary)
	initPrimary(primary)

	for i, m := range mixins {
		skipped = append(skipped, mergeSwaggerProps(primary, m)...)

		skipped = append(skipped, mergeTags(primary, m)...)

		skipped = append(skipped, mergeSecurityDefinitions(primary, m)...)

		skipped = append(skipped, mergeSecurityRequirements(primary, m)...)

		skipped = append(skipped, mergeDefinitions(primary, m)...)

		// merging paths requires a map of operationIDs to work with
		skipped = append(skipped, mergePaths(primary, m, opIDs, i)...)

		skipped = append(skipped, mergeParameters(primary, m)...)

		skipped = append(skipped, mergeResponses(primary, m)...)
	}

	return skipped
}

// getOpIDs extracts all the paths.<path>.operationIds from the given
// spec and returns them as the keys in a map with 'true' values.
func getOpIDs(s *spec.Swagger) map[string]bool {
	rv := make(map[string]bool)
	if s.Paths == nil {
		return rv
	}

	for _, v := range s.Paths.Paths {
		piops := pathItemOps(v)

		for _, op := range piops {
			rv[op.ID] = true
		}
	}

	return rv
}

func pathItemOps(p spec.PathItem) []*spec.Operation {
	var rv []*spec.Operation
	rv = appendOp(rv, p.Get)
	rv = appendOp(rv, p.Put)
	rv = appendOp(rv, p.Post)
	rv = appendOp(rv, p.Delete)
	rv = appendOp(rv, p.Head)
	rv = appendOp(rv, p.Patch)

	return rv
}

func appendOp(ops []*spec.Operation, op *spec.Operation) []*spec.Operation {
	if op == nil {
		return ops
	}

	return append(ops, op)
}

func mergeSecurityDefinitions(primary *spec.Swagger, m *spec.Swagger) (skipped []string) {
	if m.Components == nil || len(m.Components.SecuritySchemes) == 0 {
		return
	}

	if primary.Components == nil {
		primary.Components = &spec.Components{}
	}
	if primary.Components.SecuritySchemes == nil {
		primary.Components.SecuritySchemes = make(map[string]spec.SecurityScheme)
	}

	for k, v := range m.Components.SecuritySchemes {
		if _, exists := primary.Components.SecuritySchemes[k]; exists {
			warn := fmt.Sprintf(
				"SecuritySchemes entry '%v' already exists in primary or higher priority mixin, skipping\n", k)
			skipped = append(skipped, warn)

			continue
		}

		primary.Components.SecuritySchemes[k] = v
	}

	return
}

func mergeSecurityRequirements(primary *spec.Swagger, m *spec.Swagger) (skipped []string) {
	for _, v := range m.Security {
		found := false
		for _, vv := range primary.Security {
			if reflect.DeepEqual(v, vv) {
				found = true

				break
			}
		}

		if found {
			warn := fmt.Sprintf(
				"Security requirement: '%v' already exists in primary or higher priority mixin, skipping\n", v)
			skipped = append(skipped, warn)

			continue
		}
		primary.Security = append(primary.Security, v)
	}

	return
}

func mergeDefinitions(primary *spec.Swagger, m *spec.Swagger) (skipped []string) {
	if m.Components == nil || len(m.Components.Schemas) == 0 {
		return
	}

	if primary.Components == nil {
		primary.Components = &spec.Components{}
	}
	if primary.Components.Schemas == nil {
		primary.Components.Schemas = make(map[string]spec.Schema)
	}

	for k, v := range m.Components.Schemas {
		// assume name collisions represent IDENTICAL type. careful.
		if _, exists := primary.Components.Schemas[k]; exists {
			warn := fmt.Sprintf(
				"schemas entry '%v' already exists in primary or higher priority mixin, skipping\n", k)
			skipped = append(skipped, warn)

			continue
		}
		primary.Components.Schemas[k] = v
	}

	return
}

func mergePaths(primary *spec.Swagger, m *spec.Swagger, opIDs map[string]bool, mixIndex int) (skipped []string) {
	if m.Paths != nil {
		for k, v := range m.Paths.Paths {
			if _, exists := primary.Paths.Paths[k]; exists {
				warn := fmt.Sprintf(
					"paths entry '%v' already exists in primary or higher priority mixin, skipping\n", k)
				skipped = append(skipped, warn)

				continue
			}

			// Swagger requires that operationIds be
			// unique within a spec. If we find a
			// collision we append "Mixin0" to the
			// operatoinId we are adding, where 0 is mixin
			// index.  We assume that operationIds with
			// all the proivded specs are already unique.
			piops := pathItemOps(v)
			for _, piop := range piops {
				if opIDs[piop.ID] {
					piop.ID = fmt.Sprintf("%v%v%v", piop.ID, "Mixin", mixIndex)
				}
				opIDs[piop.ID] = true
			}
			primary.Paths.Paths[k] = v
		}
	}

	return
}

func mergeParameters(primary *spec.Swagger, m *spec.Swagger) (skipped []string) {
	if m.Components == nil || len(m.Components.Parameters) == 0 {
		return
	}

	if primary.Components == nil {
		primary.Components = &spec.Components{}
	}
	if primary.Components.Parameters == nil {
		primary.Components.Parameters = make(map[string]spec.Parameter)
	}

	for k, v := range m.Components.Parameters {
		// could try to rename on conflict but would
		// have to fix $refs in the mixin. Complain
		// for now
		if _, exists := primary.Components.Parameters[k]; exists {
			warn := fmt.Sprintf(
				"components parameters entry '%v' already exists in primary or higher priority mixin, skipping\n", k)
			skipped = append(skipped, warn)

			continue
		}
		primary.Components.Parameters[k] = v
	}

	return
}

func mergeResponses(primary *spec.Swagger, m *spec.Swagger) (skipped []string) {
	if m.Components == nil || len(m.Components.Responses) == 0 {
		return
	}

	if primary.Components == nil {
		primary.Components = &spec.Components{}
	}
	if primary.Components.Responses == nil {
		primary.Components.Responses = make(map[string]spec.Response)
	}

	for k, v := range m.Components.Responses {
		// could try to rename on conflict but would
		// have to fix $refs in the mixin. Complain
		// for now
		if _, exists := primary.Components.Responses[k]; exists {
			warn := fmt.Sprintf(
				"components responses entry '%v' already exists in primary or higher priority mixin, skipping\n", k)
			skipped = append(skipped, warn)

			continue
		}
		primary.Components.Responses[k] = v
	}

	return skipped
}

func mergeTags(primary *spec.Swagger, m *spec.Swagger) (skipped []string) {
	for _, v := range m.Tags {
		found := false
		for _, vv := range primary.Tags {
			if v.Name == vv.Name {
				found = true

				break
			}
		}

		if found {
			warn := fmt.Sprintf(
				"top level tags entry with name '%v' already exists in primary or higher priority mixin, skipping\n",
				v.Name,
			)
			skipped = append(skipped, warn)

			continue
		}

		primary.Tags = append(primary.Tags, v)
	}

	return
}

func mergeSwaggerProps(primary *spec.Swagger, m *spec.Swagger) []string {
	var skipped, skippedInfo, skippedDocs []string

	primary.Extensions, skipped = mergeExtensions(primary.Extensions, m.Extensions)

	if primary.Info == nil {
		primary.Info = m.Info
	} else if m.Info != nil {
		skippedInfo = mergeInfo(primary.Info, m.Info)
		skipped = append(skipped, skippedInfo...)
	}

	if primary.ExternalDocs == nil {
		primary.ExternalDocs = m.ExternalDocs
	} else if m != nil {
		skippedDocs = mergeExternalDocs(primary.ExternalDocs, m.ExternalDocs)
		skipped = append(skipped, skippedDocs...)
	}

	return skipped
}

//nolint:unparam
func mergeExternalDocs(primary *spec.ExternalDocumentation, m *spec.ExternalDocumentation) []string {
	if primary.Description == "" {
		primary.Description = m.Description
	}

	if primary.URL == "" {
		primary.URL = m.URL
	}

	return nil
}

func mergeInfo(primary *spec.Info, m *spec.Info) []string {
	var sk, skipped []string

	primary.Extensions, sk = mergeExtensions(primary.Extensions, m.Extensions)
	skipped = append(skipped, sk...)

	if primary.Description == "" {
		primary.Description = m.Description
	}

	if primary.Title == "" {
		primary.Title = m.Title
	}

	if primary.TermsOfService == "" {
		primary.TermsOfService = m.TermsOfService
	}

	if primary.Version == "" {
		primary.Version = m.Version
	}

	if primary.Contact == nil {
		primary.Contact = m.Contact
	} else if m.Contact != nil {
		var csk []string
		primary.Contact.Extensions, csk = mergeExtensions(primary.Contact.Extensions, m.Contact.Extensions)
		skipped = append(skipped, csk...)

		if primary.Contact.Name == "" {
			primary.Contact.Name = m.Contact.Name
		}

		if primary.Contact.URL == "" {
			primary.Contact.URL = m.Contact.URL
		}

		if primary.Contact.Email == "" {
			primary.Contact.Email = m.Contact.Email
		}
	}

	if primary.License == nil {
		primary.License = m.License
	} else if m.License != nil {
		var lsk []string
		primary.License.Extensions, lsk = mergeExtensions(primary.License.Extensions, m.License.Extensions)
		skipped = append(skipped, lsk...)

		if primary.License.Name == "" {
			primary.License.Name = m.License.Name
		}

		if primary.License.URL == "" {
			primary.License.URL = m.License.URL
		}
	}

	return skipped
}

func mergeExtensions(primary spec.Extensions, m spec.Extensions) (result spec.Extensions, skipped []string) {
	if primary == nil {
		result = m

		return
	}

	if m == nil {
		result = primary

		return
	}

	result = primary
	for k, v := range m {
		if _, found := primary[k]; found {
			skipped = append(skipped, k)

			continue
		}

		primary[k] = v
	}

	return
}

func initPrimary(primary *spec.Swagger) {
	if primary.Components == nil {
		primary.Components = &spec.Components{}
	}

	if primary.Components.SecuritySchemes == nil {
		primary.Components.SecuritySchemes = make(map[string]spec.SecurityScheme)
	}

	if primary.Security == nil {
		primary.Security = make([]map[string][]string, 0, allocSmallMap)
	}

	if primary.Tags == nil {
		primary.Tags = make([]spec.Tag, 0, allocSmallMap)
	}

	if primary.Paths == nil {
		primary.Paths = &spec.Paths{Paths: make(map[string]spec.PathItem)}
	}

	if primary.Paths.Paths == nil {
		primary.Paths.Paths = make(map[string]spec.PathItem)
	}

	if primary.Components.Schemas == nil {
		primary.Components.Schemas = make(map[string]spec.Schema)
	}

	if primary.Components.Parameters == nil {
		primary.Components.Parameters = make(map[string]spec.Parameter)
	}

	if primary.Components.Responses == nil {
		primary.Components.Responses = make(map[string]spec.Response)
	}
}

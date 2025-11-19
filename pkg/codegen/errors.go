// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package codegen

import "errors"

var (
	ErrOperationNameEmpty                        = errors.New("operation name cannot be an empty string")
	ErrRequestPathEmpty                          = errors.New("request path cannot be an empty string")
	ErrMergingSchemasWithDifferentUniqueItems    = errors.New("merging two schemas with different UniqueItems")
	ErrMergingSchemasWithDifferentExclusiveMin   = errors.New("merging two schemas with different ExclusiveMin")
	ErrMergingSchemasWithDifferentExclusiveMax   = errors.New("merging two schemas with different ExclusiveMax")
	ErrMergingSchemasWithDifferentNullable       = errors.New("merging two schemas with different Nullable")
	ErrMergingSchemasWithDifferentReadOnly       = errors.New("merging two schemas with different ReadOnly")
	ErrMergingSchemasWithDifferentWriteOnly      = errors.New("merging two schemas with different WriteOnly")
	ErrTransitiveMergingAllOfSchema1             = errors.New("error transitive merging AllOf on schema 1")
	ErrTransitiveMergingAllOfSchema2             = errors.New("error transitive merging AllOf on schema 2")
	ErrMergingSchemasWithDifferentDefaults       = errors.New("merging two sets of defaults is undefined")
	ErrMergingSchemasWithDifferentFormats        = errors.New("can not merge incompatible formats")
	ErrMergingSchemasWithDifferentDiscriminators = errors.New("merging two schemas with discriminators is not supported")
	ErrMergingSchemasWithAdditionalProperties    = errors.New("merging two schemas with additional properties, this is unhandled")
	ErrAmbiguousDiscriminatorMapping             = errors.New("ambiguous discriminator.mapping: please replace inlined object with $ref")
	ErrDiscriminatorNotAllMapped                 = errors.New("discriminator: not all schemas were mapped")
	ErrEmptySchema                               = errors.New("empty schema")
)

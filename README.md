# `oapi-codegen`

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/9450/badge)](https://www.bestpractices.dev/projects/9450)

Using `oapi-codegen` allows you to reduce the boilerplate required to create or integrate with 
services based on [OpenAPI 3.x](https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.0.md), and instead focus on writing your business logic, and working
on the real value-add for your organisation.

With `oapi-codegen`, there are a few [Key Design Decisions](#key-design-decisions) we've made, 
including:

- idiomatic Go, where possible
- fairly simple generated code, erring on the side of duplicate code over nicely refactored code
- supporting as much of OpenAPI 3.x as is possible, alongside Go's type system


## Migrate from v2

This project is a fork of [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) v2.  
Due to the lack of OpenAPI 3.1 support in the original repository, we introduced a fully reworked implementation.  
While this includes some breaking changes, it also brings more flexible generator and parser APIs for finer control over code generation.  
If you're migrating from v2, please refer to the [migration guide](docs/migrate-from-v2.md) for important differences.

## Usage

`oapi-codegen` is largely configured using a YAML configuration file, to simplify the number of 
flags that users need to remember, and to make reading the `go:generate` command less daunting.

## Features

At a high level, `oapi-codegen` supports:

- Generating the types ([docs](#generating-api-models))
- Splitting large OpenAPI specs across multiple packages([docs](#import-mapping))
  - This is also known as "Import Mapping" or "external references" across our documentation / discussion in GitHub issues


## Key design decisions

- Produce an interface that can be satisfied by your implementation, with reduced boilerplate
- Bulk processing and parsing of OpenAPI document in Go
- Resulting output is using Go's `text/template`s, which are user-overridable
- Attempts to produce Idiomatic Go
- Single or multiple file output
- Support of OpenAPI 3.1
- Extract parameters from requests, to reduce work required by your implementation
- Implicit `additionalProperties` are ignored by default ([more details](#additional-properties-additionalproperties))
- Prune unused types by default



## Generating API models

If you're looking to only generate the models for interacting with a remote service, 
for instance if you need to hand-roll the API client for whatever reason, you can do this as-is.

> [!TIP]
> Try to define as much as possible within the `#/components/schemas` object, as `oapi-codegen` 
> will generate all the types here.
>
> Although we can generate some types based on inline definitions in i.e. a path's response type, 
> it isn't always possible to do this, or if it is generated, can be a little awkward to work 
> with as it may be defined as an anonymous struct.


## OpenAPI extensions

As well as the core OpenAPI support, we also support the following OpenAPI extensions, 
as denoted by the [OpenAPI Specification Extensions](https://spec.openapis.org/oas/v3.0.3#specification-extensions).

<table>

<tr>
<th>
Extension
</th>
<th>
Description
</th>
<th>
Example usage
</th>
</tr>

<tr>
<td>

`x-go-type` <br>
`x-go-type-import`

</td>
<td>
Override the generated type definition (and optionally, add an import from another package)
</td>
<td>
<details>

Using the `x-go-type` (and optionally, `x-go-type-import` when you need to import another package)
allows overriding the type that `oapi-codegen` determined the generated type should be.

We can see this at play with the following schemas:

```yaml
components:
  schemas:
    Client:
      type: object
      required:
        - name
      properties:
        name:
          type: string
        id:
          type: number
    ClientWithExtension:
      type: object
      required:
        - name
      properties:
        name:
          type: string
          # this is a bit of a contrived example, as you could instead use
          # `format: uuid` but it explains how you'd do this when there may be
          # a clash, for instance if you already had a `uuid` package that was
          # being imported, or ...
          x-go-type: googleuuid.UUID
          x-go-type-import:
            path: github.com/google/uuid
            name: googleuuid
        id:
          type: number
          # ... this is also a bit of a contrived example, as you could use
          # `type: integer` but in the case that you know better than what
          # oapi-codegen is generating, like so:
          x-go-type: int64
```

From here, we now get two different models:

```go
// Client defines model for Client.
type Client struct {
	Id   *float32 `json:"id,omitempty"`
	Name string   `json:"name"`
}

// ClientWithExtension defines model for ClientWithExtension.
type ClientWithExtension struct {
	Id   *int64          `json:"id,omitempty"`
	Name googleuuid.UUID `json:"name"`
}
```

You can see this in more detail in [the example code](examples/extensions/xgotype/).

</details>
</td>
</tr>

<tr>
<td>

`x-go-type-skip-optional-pointer`

</td>
<td>
Do not add a pointer type for optional fields in structs
</td>
<td>
<details>

By default, `oapi-codegen` will generate a pointer for optional fields.

Using the `x-go-type-skip-optional-pointer` extension allows omitting that pointer.

We can see this at play with the following schemas:

```yaml
components:
  schemas:
    Client:
      type: object
      required:
        - name
      properties:
        name:
          type: string
        id:
          type: number
    ClientWithExtension:
      type: object
      required:
        - name
      properties:
        name:
          type: string
        id:
          type: number
          x-go-type-skip-optional-pointer: true
```

From here, we now get two different models:

```go
// Client defines model for Client.
type Client struct {
	Id   *float32 `json:"id,omitempty"`
	Name string   `json:"name"`
}

// ClientWithExtension defines model for ClientWithExtension.
type ClientWithExtension struct {
	Id   float32 `json:"id,omitempty"`
	Name string  `json:"name"`
}
```

You can see this in more detail in [the example code](examples/extensions/xgotypeskipoptionalpointer/).

</details>
</td>
</tr>

<tr>
<td>

`x-go-name`

</td>
<td>
Override the generated name of a field or a type
</td>
<td>
<details>

By default, `oapi-codegen` will attempt to generate the name of fields and types in as best a way 
it can.

However, sometimes, the name doesn't quite fit what your codebase standards are, or the intent 
of the field, so you can override it with `x-go-name`.

We can see this at play with the following schemas:

```yaml
openapi: "3.0.0"
info:
  version: 1.0.0
  title: x-go-name
components:
  schemas:
    Client:
      type: object
      required:
        - name
      properties:
        name:
          type: string
        id:
          type: number
    ClientWithExtension:
      type: object
      # can be used on a type
      x-go-name: ClientRenamedByExtension
      required:
        - name
      properties:
        name:
          type: string
        id:
          type: number
          # or on a field
          x-go-name: AccountIdentifier
```

From here, we now get two different models:

```go
// Client defines model for Client.
type Client struct {
	Id   *float32 `json:"id,omitempty"`
	Name string   `json:"name"`
}

// ClientRenamedByExtension defines model for ClientWithExtension.
type ClientRenamedByExtension struct {
	AccountIdentifier *float32 `json:"id,omitempty"`
	Name              string   `json:"name"`
}
```

You can see this in more detail in [the example code](examples/extensions/xgoname/).

</details>
</td>
</tr>

<tr>
<td>

`x-go-type-name`

</td>
<td>
Override the generated name of a type
</td>
<td>
<details>

> [!NOTE]
> Notice that this is subtly different to the `x-go-name`, which also applies to _fields_ within `struct`s.

By default, `oapi-codegen` will attempt to generate the name of types in as best a way it can.

However, sometimes, the name doesn't quite fit what your codebase standards are, or the intent of the field, so you can override it with `x-go-name`.

We can see this at play with the following schemas:

```yaml
openapi: "3.0.0"
info:
  version: 1.0.0
  title: x-go-type-name
components:
  schemas:
    Client:
      type: object
      required:
        - name
      properties:
        name:
          type: string
        id:
          type: number
    ClientWithExtension:
      type: object
      x-go-type-name: ClientRenamedByExtension
      required:
        - name
      properties:
        name:
          type: string
        id:
          type: number
          # NOTE attempting a `x-go-type-name` here is a no-op, as we're not producing a _type_ only a _field_
          x-go-type-name: ThisWillNotBeUsed
```

From here, we now get two different models and a type alias:

```go
// Client defines model for Client.
type Client struct {
	Id   *float32 `json:"id,omitempty"`
	Name string   `json:"name"`
}

// ClientWithExtension defines model for ClientWithExtension.
type ClientWithExtension = ClientRenamedByExtension

// ClientRenamedByExtension defines model for .
type ClientRenamedByExtension struct {
	Id   *float32 `json:"id,omitempty"`
	Name string   `json:"name"`
}
```

You can see this in more detail in [the example code](examples/extensions/xgotypename/).

</details>
</td>
</tr>

<tr>
<td>

`x-omitempty`

</td>
<td>
Force the presence of the JSON tag `omitempty` on a field
</td>
<td>
<details>

In a case that you may want to add the JSON struct tag `omitempty` to types that don't have one generated by default - for instance a required field - you can use the `x-omitempty` extension.

We can see this at play with the following schemas:

```yaml
openapi: "3.0.0"
info:
  version: 1.0.0
  title: x-omitempty
components:
  schemas:
    Client:
      type: object
      required:
        - name
      properties:
        name:
          type: string
        id:
          type: number
    ClientWithExtension:
      type: object
      required:
        - name
      properties:
        name:
          type: string
          # for some reason, you may want this behavior, even though it's a required field
          x-omitempty: true
        id:
          type: number
```

From here, we now get two different models:

```go
// Client defines model for Client.
type Client struct {
	Id   *float32 `json:"id,omitempty"`
	Name string   `json:"name"`
}

// ClientWithExtension defines model for ClientWithExtension.
type ClientWithExtension struct {
	Id   *float32 `json:"id,omitempty"`
	Name string   `json:"name,omitempty"`
}
```

You can see this in more detail in [the example code](examples/extensions/xomitempty/).

</details>
</td>
</tr>

<tr>
<td>

`x-go-json-ignore`

</td>
<td>
When (un)marshaling JSON, ignore field(s)
</td>
<td>
<details>

By default, `oapi-codegen` will generate `json:"..."` struct tags for all fields in a struct, so JSON (un)marshaling works.

However, sometimes, you want to omit fields, which can be done with the `x-go-json-ignore` extension.

We can see this at play with the following schemas:

```yaml
openapi: "3.0.0"
info:
  version: 1.0.0
  title: x-go-json-ignore
components:
  schemas:
    Client:
      type: object
      required:
        - name
      properties:
        name:
          type: string
        complexField:
          type: object
          properties:
            name:
              type: string
            accountName:
              type: string
          # ...
    ClientWithExtension:
      type: object
      required:
        - name
      properties:
        name:
          type: string
        complexField:
          type: object
          properties:
            name:
              type: string
            accountName:
              type: string
          # ...
          x-go-json-ignore: true
```

From here, we now get two different models:

```go
// Client defines model for Client.
type Client struct {
	ComplexField *struct {
		AccountName *string `json:"accountName,omitempty"`
		Name        *string `json:"name,omitempty"`
	} `json:"complexField,omitempty"`
	Name string `json:"name"`
}

// ClientWithExtension defines model for ClientWithExtension.
type ClientWithExtension struct {
	ComplexField *struct {
		AccountName *string `json:"accountName,omitempty"`
		Name        *string `json:"name,omitempty"`
	} `json:"-"`
	Name string `json:"name"`
}
```

Notice that the `ComplexField` is still generated in full, but the type will then be ignored with JSON marshalling.

You can see this in more detail in [the example code](examples/extensions/xgojsonignore/).

</details>
</td>
</tr>

<tr>
<td>

`x-oapi-codegen-extra-tags`

</td>
<td>
Generate arbitrary struct tags to fields
</td>
<td>
<details>

If you're making use of a field's struct tags to i.e. apply validation, decide whether something should be logged, etc, you can use `x-oapi-codegen-extra-tags` to set additional tags for your generated types.

We can see this at play with the following schemas:

```yaml
openapi: "3.0.0"
info:
  version: 1.0.0
  title: x-oapi-codegen-extra-tags
components:
  schemas:
    Client:
      type: object
      required:
        - name
        - id
      properties:
        name:
          type: string
        id:
          type: number
    ClientWithExtension:
      type: object
      required:
        - name
        - id
      properties:
        name:
          type: string
        id:
          type: number
          x-oapi-codegen-extra-tags:
            validate: "required,min=1,max=256"
            safe-to-log: "true"
            gorm: primarykey
```

From here, we now get two different models:

```go
// Client defines model for Client.
type Client struct {
	Id   float32 `json:"id"`
	Name string  `json:"name"`
}

// ClientWithExtension defines model for ClientWithExtension.
type ClientWithExtension struct {
	Id   float32 `gorm:"primarykey" json:"id" safe-to-log:"true" validate:"required,min=1,max=256"`
	Name string  `json:"name"`
}
```

You can see this in more detail in [the example code](examples/extensions/xoapicodegenextratags/).

</details>
</td>
</tr>

<tr>
<td>

`x-enum-names`

</td>
<td>
Override generated variable names for enum constants
</td>
<td>
<details>

When consuming an enum value from an external system, the name may not produce a nice variable name. 
Using the `x-enum-names` extension allows overriding the name of the generated variable names.

We can see this at play with the following schemas:

```yaml
openapi: "3.0.0"
info:
  version: 1.0.0
  title: x-enum-names
components:
  schemas:
    ClientType:
      type: string
      enum:
        - ACT
        - EXP
    ClientTypeWithNamesExtension:
      type: string
      enum:
        - ACT
        - EXP
      x-enum-names:
        - Active
        - Expired
```

From here, we now get two different forms of the same enum definition.

```go
// Defines values for ClientType.
const (
	ACT ClientType = "ACT"
	EXP ClientType = "EXP"
)

// ClientType defines model for ClientType.
type ClientType string

// Defines values for ClientTypeWithExtension.
const (
	Active  ClientTypeWithExtension = "ACT"
	Expired ClientTypeWithExtension = "EXP"
)

// ClientTypeWithExtension defines model for ClientTypeWithExtension.
type ClientTypeWithExtension string
```

You can see this in more detail in [the example code](examples/extensions/xenumnames/).

</details>
</td>
</tr>

<tr>
<td>

`x-deprecated-reason`

</td>
<td>
Add a GoDoc deprecation warning to a type
</td>
<td>
<details>

When an OpenAPI type is deprecated, a deprecation warning can be added in the GoDoc 
using `x-deprecated-reason`.

We can see this at play with the following schemas:

```yaml
openapi: "3.0.0"
info:
  version: 1.0.0
  title: x-deprecated-reason
components:
  schemas:
    Client:
      type: object
      required:
        - name
      properties:
        name:
          type: string
        id:
          type: number
    ClientWithExtension:
      type: object
      required:
        - name
      properties:
        name:
          type: string
          deprecated: true
          x-deprecated-reason: Don't use because reasons
        id:
          type: number
          # NOTE that this doesn't generate, as no `deprecated: true` is set
          x-deprecated-reason: NOTE you shouldn't see this, as you've not deprecated this field
```

From here, we now get two different forms of the same enum definition.

```go
// Client defines model for Client.
type Client struct {
	Id   *float32 `json:"id,omitempty"`
	Name string   `json:"name"`
}

// ClientWithExtension defines model for ClientWithExtension.
type ClientWithExtension struct {
	Id *float32 `json:"id,omitempty"`
	// Deprecated: Don't use because reasons
	Name string `json:"name"`
}
```

Notice that because we've not set `deprecated: true` to the `name` field, it doesn't generate 
a deprecation warning.

You can see this in more detail in [the example code](examples/extensions/xdeprecatedreason/).

</details>
</td>
</tr>

</table>

## Custom code generation

It is possible to extend the inbuilt code generation from `oapi-codegen` using 
Go's `text/template`s.

You can specify, through your configuration file, the `user-templates` setting to override 
the inbuilt templates and use a user-defined template.

> [!NOTE]
> Filenames given to the `user-templates` configuration must **exactly** match the filename 
> that `oapi-codegen` is looking for

### Local paths

Within your configuration file, you can specify relative or absolute paths to a file to 
reference for the template, such as:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/doordash/oapi-codegen/HEAD/configuration-schema.json
# ...
user-templates:
  client.tmpl: ./custom-template.tmpl
  enums.tmpl: /tmp/foo.bar
  types.tmpl: no-prefix.tmpl
```

### Using the Go package

You can get full control of the generator and the parser by using the `codegen` package directly.

TBD: add documentation

## Additional Properties (`additionalProperties`)

[OpenAPI Schemas](https://spec.openapis.org/oas/v3.0.3.html#schema-object) implicitly accept `additionalProperties`, meaning that any fields 
provided, but not explicitly defined via properties on the schema are accepted as input, 
and propagated. 
When unspecified, OpenAPI defines that the `additionalProperties` field is assumed to be `true`.

For simplicity, and to remove a fair bit of duplication and boilerplate, 
`oapi-codegen` decides to ignore the implicit `additionalProperties: true`,
and instead requires you to specify the `additionalProperties` key to generate the boilerplate.

Below you can see some examples of how `additionalProperties` affects the generated code.

### Implicit `additionalProperties: true` / no `additionalProperties` set

```yaml
components:
  schemas:
    Thing:
      type: object
      required:
        - id
      properties:
        id:
          type: integer
      # implicit additionalProperties: true
```

Will generate:

```go
// Thing defines model for Thing.
type Thing struct {
	Id int `json:"id"`
}

// with no generated boilerplate nor the `AdditionalProperties` field
```

### Explicit `additionalProperties: true`

```yaml
components:
  schemas:
    Thing:
      type: object
      required:
        - id
      properties:
        id:
          type: integer
      # explicit true
      additionalProperties: true
```

Will generate:

```go
// Thing defines model for Thing.
type Thing struct {
	Id                   int                    `json:"id"`
	AdditionalProperties map[string]interface{} `json:"-"`
}

// with generated boilerplate below
```

<details>

<summary>Boilerplate</summary>

```go

// Getter for additional properties for Thing. Returns the specified
// element and whether it was found
func (a Thing) Get(fieldName string) (value interface{}, found bool) {
	if a.AdditionalProperties != nil {
		value, found = a.AdditionalProperties[fieldName]
	}
	return
}

// Setter for additional properties for Thing
func (a *Thing) Set(fieldName string, value interface{}) {
	if a.AdditionalProperties == nil {
		a.AdditionalProperties = make(map[string]interface{})
	}
	a.AdditionalProperties[fieldName] = value
}

// Override default JSON handling for Thing to handle AdditionalProperties
func (a *Thing) UnmarshalJSON(b []byte) error {
	object := make(map[string]json.RawMessage)
	err := json.Unmarshal(b, &object)
	if err != nil {
		return err
	}

	if raw, found := object["id"]; found {
		err = json.Unmarshal(raw, &a.Id)
		if err != nil {
			return fmt.Errorf("error reading 'id': %w", err)
		}
		delete(object, "id")
	}

	if len(object) != 0 {
		a.AdditionalProperties = make(map[string]interface{})
		for fieldName, fieldBuf := range object {
			var fieldVal interface{}
			err := json.Unmarshal(fieldBuf, &fieldVal)
			if err != nil {
				return fmt.Errorf("error unmarshaling field %s: %w", fieldName, err)
			}
			a.AdditionalProperties[fieldName] = fieldVal
		}
	}
	return nil
}

// Override default JSON handling for Thing to handle AdditionalProperties
func (a Thing) MarshalJSON() ([]byte, error) {
	var err error
	object := make(map[string]json.RawMessage)

	object["id"], err = json.Marshal(a.Id)
	if err != nil {
		return nil, fmt.Errorf("error marshaling 'id': %w", err)
	}

	for fieldName, field := range a.AdditionalProperties {
		object[fieldName], err = json.Marshal(field)
		if err != nil {
			return nil, fmt.Errorf("error marshaling '%s': %w", fieldName, err)
		}
	}
	return json.Marshal(object)
}
```

</details>


### `additionalProperties` as `integer`s

```yaml
components:
  schemas:
    Thing:
      type: object
      required:
        - id
      properties:
        id:
          type: integer
      # simple type
      additionalProperties:
        type: integer
```

Will generate:

```go
// Thing defines model for Thing.
type Thing struct {
	Id                   int            `json:"id"`
	AdditionalProperties map[string]int `json:"-"`
}

// with generated boilerplate below
```

<details>

<summary>Boilerplate</summary>

```go
// Getter for additional properties for Thing. Returns the specified
// element and whether it was found
func (a Thing) Get(fieldName string) (value int, found bool) {
	if a.AdditionalProperties != nil {
		value, found = a.AdditionalProperties[fieldName]
	}
	return
}

// Setter for additional properties for Thing
func (a *Thing) Set(fieldName string, value int) {
	if a.AdditionalProperties == nil {
		a.AdditionalProperties = make(map[string]int)
	}
	a.AdditionalProperties[fieldName] = value
}

// Override default JSON handling for Thing to handle AdditionalProperties
func (a *Thing) UnmarshalJSON(b []byte) error {
	object := make(map[string]json.RawMessage)
	err := json.Unmarshal(b, &object)
	if err != nil {
		return err
	}

	if raw, found := object["id"]; found {
		err = json.Unmarshal(raw, &a.Id)
		if err != nil {
			return fmt.Errorf("error reading 'id': %w", err)
		}
		delete(object, "id")
	}

	if len(object) != 0 {
		a.AdditionalProperties = make(map[string]int)
		for fieldName, fieldBuf := range object {
			var fieldVal int
			err := json.Unmarshal(fieldBuf, &fieldVal)
			if err != nil {
				return fmt.Errorf("error unmarshaling field %s: %w", fieldName, err)
			}
			a.AdditionalProperties[fieldName] = fieldVal
		}
	}
	return nil
}

// Override default JSON handling for Thing to handle AdditionalProperties
func (a Thing) MarshalJSON() ([]byte, error) {
	var err error
	object := make(map[string]json.RawMessage)

	object["id"], err = json.Marshal(a.Id)
	if err != nil {
		return nil, fmt.Errorf("error marshaling 'id': %w", err)
	}

	for fieldName, field := range a.AdditionalProperties {
		object[fieldName], err = json.Marshal(field)
		if err != nil {
			return nil, fmt.Errorf("error marshaling '%s': %w", fieldName, err)
		}
	}
	return json.Marshal(object)
}
```

</details>

### `additionalProperties` with an object

```yaml
components:
  schemas:
    Thing:
      type: object
      required:
        - id
      properties:
        id:
          type: integer
      # object
      additionalProperties:
        type: object
        properties:
          foo:
            type: string
```

Will generate:

```go
// Thing defines model for Thing.
type Thing struct {
	Id                   int `json:"id"`
	AdditionalProperties map[string]struct {
		Foo *string `json:"foo,omitempty"`
	} `json:"-"`
}

// with generated boilerplate below
```

<details>

<summary>Boilerplate</summary>

```go
// Getter for additional properties for Thing. Returns the specified
// element and whether it was found
func (a Thing) Get(fieldName string) (value struct {
	Foo *string `json:"foo,omitempty"`
}, found bool) {
	if a.AdditionalProperties != nil {
		value, found = a.AdditionalProperties[fieldName]
	}
	return
}

// Setter for additional properties for Thing
func (a *Thing) Set(fieldName string, value struct {
	Foo *string `json:"foo,omitempty"`
}) {
	if a.AdditionalProperties == nil {
		a.AdditionalProperties = make(map[string]struct {
			Foo *string `json:"foo,omitempty"`
		})
	}
	a.AdditionalProperties[fieldName] = value
}

// Override default JSON handling for Thing to handle AdditionalProperties
func (a *Thing) UnmarshalJSON(b []byte) error {
	object := make(map[string]json.RawMessage)
	err := json.Unmarshal(b, &object)
	if err != nil {
		return err
	}

	if raw, found := object["id"]; found {
		err = json.Unmarshal(raw, &a.Id)
		if err != nil {
			return fmt.Errorf("error reading 'id': %w", err)
		}
		delete(object, "id")
	}

	if len(object) != 0 {
		a.AdditionalProperties = make(map[string]struct {
			Foo *string `json:"foo,omitempty"`
		})
		for fieldName, fieldBuf := range object {
			var fieldVal struct {
				Foo *string `json:"foo,omitempty"`
			}
			err := json.Unmarshal(fieldBuf, &fieldVal)
			if err != nil {
				return fmt.Errorf("error unmarshaling field %s: %w", fieldName, err)
			}
			a.AdditionalProperties[fieldName] = fieldVal
		}
	}
	return nil
}

// Override default JSON handling for Thing to handle AdditionalProperties
func (a Thing) MarshalJSON() ([]byte, error) {
	var err error
	object := make(map[string]json.RawMessage)

	object["id"], err = json.Marshal(a.Id)
	if err != nil {
		return nil, fmt.Errorf("error marshaling 'id': %w", err)
	}

	for fieldName, field := range a.AdditionalProperties {
		object[fieldName], err = json.Marshal(field)
		if err != nil {
			return nil, fmt.Errorf("error marshaling '%s': %w", fieldName, err)
		}
	}
	return json.Marshal(object)
}
```

</details>


## Examples

The [examples directory](examples) contains some additional cases which are useful examples 
for how to use `oapi-codegen`, including how you'd take the Petstore API and implement it 
with `oapi-codegen`.


## Frequently Asked Questions (FAQs)

### How does `oapi-codegen` handle `anyOf`, `allOf` and `oneOf`?

`oapi-codegen` supports `anyOf`, `allOf` and `oneOf` for generated code.

For instance, through the following OpenAPI spec:

```yaml
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Using complex schemas
  description: An example of `anyOf`, `allOf` and `oneOf`
components:
  schemas:
    # base types
    Client:
      type: object
      required:
        - name
      properties:
        name:
          type: string
    Identity:
      type: object
      required:
        - issuer
      properties:
        issuer:
          type: string

    # allOf performs a union of all types defined
    ClientWithId:
      allOf:
        - $ref: '#/components/schemas/Client'
        - properties:
            id:
              type: integer
          required:
            - id

    # allOf performs a union of all types defined, but if there's a duplicate field defined, it'll be overwritten by the last schema
    # https://github.com/oapi-codegen/oapi-codegen/issues/1569
    IdentityWithDuplicateField:
      allOf:
        # `issuer` will be ignored
        - $ref: '#/components/schemas/Identity'
        # `issuer` will be ignored
        - properties:
            issuer:
              type: integer
        # `issuer` will take precedence
        - properties:
            issuer:
              type: object
              properties:
                name:
                  type: string
              required:
                - name

    # anyOf results in a type that has an `AsClient`/`MergeClient`/`FromClient` and an `AsIdentity`/`MergeIdentity`/`FromIdentity` method so you can choose which of them you want to retrieve
    ClientAndMaybeIdentity:
      anyOf:
        - $ref: '#/components/schemas/Client'
        - $ref: '#/components/schemas/Identity'

    # oneOf results in a type that has an `AsClient`/`MergeClient`/`FromClient` and an `AsIdentity`/`MergeIdentity`/`FromIdentity` method so you can choose which of them you want to retrieve
    ClientOrIdentity:
      oneOf:
        - $ref: '#/components/schemas/Client'
        - $ref: '#/components/schemas/Identity'
```

This results in the following types:

<details>

<summary>Base types</summary>

```go
// Client defines model for Client.
type Client struct {
	Name string `json:"name"`
}

// Identity defines model for Identity.
type Identity struct {
	Issuer string `json:"issuer"`
}
```

</details>

<details>

<summary><code>allOf</code></summary>

```go
// ClientWithId defines model for ClientWithId.
type ClientWithId struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

// IdentityWithDuplicateField defines model for IdentityWithDuplicateField.
type IdentityWithDuplicateField struct {
	Issuer struct {
		Name string `json:"name"`
	} `json:"issuer"`
}
```

</details>

<details>

<summary><code>anyOf</code></summary>

```go
import (
	"encoding/json"

	"github.com/oapi-codegen/runtime"
)

// ClientAndMaybeIdentity defines model for ClientAndMaybeIdentity.
type ClientAndMaybeIdentity struct {
	union json.RawMessage
}

// AsClient returns the union data inside the ClientAndMaybeIdentity as a Client
func (t ClientAndMaybeIdentity) AsClient() (Client, error) {
	var body Client
	err := json.Unmarshal(t.union, &body)
	return body, err
}

// FromClient overwrites any union data inside the ClientAndMaybeIdentity as the provided Client
func (t *ClientAndMaybeIdentity) FromClient(v Client) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

// MergeClient performs a merge with any union data inside the ClientAndMaybeIdentity, using the provided Client
func (t *ClientAndMaybeIdentity) MergeClient(v Client) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	merged, err := runtime.JSONMerge(t.union, b)
	t.union = merged
	return err
}

// AsIdentity returns the union data inside the ClientAndMaybeIdentity as a Identity
func (t ClientAndMaybeIdentity) AsIdentity() (Identity, error) {
	var body Identity
	err := json.Unmarshal(t.union, &body)
	return body, err
}

// FromIdentity overwrites any union data inside the ClientAndMaybeIdentity as the provided Identity
func (t *ClientAndMaybeIdentity) FromIdentity(v Identity) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

// MergeIdentity performs a merge with any union data inside the ClientAndMaybeIdentity, using the provided Identity
func (t *ClientAndMaybeIdentity) MergeIdentity(v Identity) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	merged, err := runtime.JSONMerge(t.union, b)
	t.union = merged
	return err
}

func (t ClientAndMaybeIdentity) MarshalJSON() ([]byte, error) {
	b, err := t.union.MarshalJSON()
	return b, err
}

func (t *ClientAndMaybeIdentity) UnmarshalJSON(b []byte) error {
	err := t.union.UnmarshalJSON(b)
	return err
}


```

</details>

<details>

<summary><code>oneOf</code></summary>

```go
// AsClient returns the union data inside the ClientOrIdentity as a Client
func (t ClientOrIdentity) AsClient() (Client, error) {
	var body Client
	err := json.Unmarshal(t.union, &body)
	return body, err
}

// FromClient overwrites any union data inside the ClientOrIdentity as the provided Client
func (t *ClientOrIdentity) FromClient(v Client) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

// MergeClient performs a merge with any union data inside the ClientOrIdentity, using the provided Client
func (t *ClientOrIdentity) MergeClient(v Client) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	merged, err := runtime.JSONMerge(t.union, b)
	t.union = merged
	return err
}

// AsIdentity returns the union data inside the ClientOrIdentity as a Identity
func (t ClientOrIdentity) AsIdentity() (Identity, error) {
	var body Identity
	err := json.Unmarshal(t.union, &body)
	return body, err
}

// FromIdentity overwrites any union data inside the ClientOrIdentity as the provided Identity
func (t *ClientOrIdentity) FromIdentity(v Identity) error {
	b, err := json.Marshal(v)
	t.union = b
	return err
}

// MergeIdentity performs a merge with any union data inside the ClientOrIdentity, using the provided Identity
func (t *ClientOrIdentity) MergeIdentity(v Identity) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	merged, err := runtime.JSONMerge(t.union, b)
	t.union = merged
	return err
}

func (t ClientOrIdentity) MarshalJSON() ([]byte, error) {
	b, err := t.union.MarshalJSON()
	return b, err
}

func (t *ClientOrIdentity) UnmarshalJSON(b []byte) error {
	err := t.union.UnmarshalJSON(b)
	return err
}
```

</details>

For more info, check out [the example code](examples/anyof-allof-oneof/).

### How can I ignore parts of the spec I don't care about?

By default, `oapi-codegen` will generate everything from the specification.

If you'd like to reduce what's generated, you can use one of a few options 
in [the configuration file](#usage) to tune the generation of the resulting output:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/doordash/oapi-codegen/HEAD/configuration-schema.json
filter:
  include:
    paths: []
    tags: []
    operation-ids: []
    schema-properties:
    extensions: []
  exclude:
    operation-ids: []
```

## License
This project is licensed under the Apache License 2.0.  
See [LICENSE.txt](LICENSE.txt) for details.

## Notices
See [NOTICE.txt](NOTICE.txt) for third-party components and attributions.

## Contributor License Agreement (CLA)
Contributions to this project require agreeing to the DoorDash Contributor License Agreement.  
See [CONTRIBUTOR_LICENSE_AGREEMENT.txt](CONTRIBUTOR_LICENSE_AGREEMENT.txt).

# `oapi-codegen`

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/9450/badge)](https://www.bestpractices.dev/projects/9450)

Using `oapi-codegen` allows you to reduce the boilerplate required to create or integrate with services based on [OpenAPI 3.x](https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.0.md), and instead focus on writing your business logic, and working on the real value-add for your organisation.

With `oapi-codegen`, there are a few [Key Design Decisions](#key-design-decisions) we've made, including:

- idiomatic Go, where possible
- fairly simple generated code, erring on the side of duplicate code over nicely refactored code
- supporting as much of OpenAPI 3.x as is possible, alongside Go's type system


## Usage

`oapi-codegen` is largely configured using a YAML configuration file, to simplify the number of flags that users need to remember, and to make reading the `go:generate` command less daunting.

For full details of what is supported, it's worth checking out [the GoDoc for `codegen.Configuration`](https://pkg.go.dev/github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen#Configuration).

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

If you're looking to only generate the models for interacting with a remote service, for instance if you need to hand-roll the API client for whatever reason, you can do this as-is.

> [!TIP]
> Try to define as much as possible within the `#/components/schemas` object, as `oapi-codegen` will generate all the types here.
>
> Although we can generate some types based on inline definitions in i.e. a path's response type, it isn't always possible to do this, or if it is generated, can be a little awkward to work with as it may be defined as an anonymous struct.


## OpenAPI extensions

As well as the core OpenAPI support, we also support the following OpenAPI extensions, as denoted by the [OpenAPI Specification Extensions](https://spec.openapis.org/oas/v3.0.3#specification-extensions).

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

Using the `x-go-type` (and optionally, `x-go-type-import` when you need to import another package) allows overriding the type that `oapi-codegen` determined the generated type should be.

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

By default, `oapi-codegen` will attempt to generate the name of fields and types in as best a way it can.

However, sometimes, the name doesn't quite fit what your codebase standards are, or the intent of the field, so you can override it with `x-go-name`.

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
          # for some reason, you may want this behaviour, even though it's a required field
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

Notice that the `ComplexField` is still generated in full, but the type will then be ignored with JSON marshalling.

You can see this in more detail in [the example code](examples/extensions/xgojsonignore/).

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

When an OpenAPI type is deprecated, a deprecation warning can be added in the GoDoc using `x-deprecated-reason`.

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

Notice that because we've not set `deprecated: true` to the `name` field, it doesn't generate a deprecation warning.

You can see this in more detail in [the example code](examples/extensions/xdeprecatedreason/).

</details>
</td>
</tr>

<tr>
<td>

`x-order`

</td>
<td>
Explicitly order struct fields
</td>
<td>
<details>

Whether you like certain fields being ordered before others, or you want to perform more efficient packing of your structs, the `x-order` extension is here for you.

Note that `x-order` is 1-indexed - `x-order: 0` is not a valid value.

When an OpenAPI type is deprecated, a deprecation warning can be added in the GoDoc using `x-deprecated-reason`.

We can see this at play with the following schemas:

```yaml
openapi: "3.0.0"
info:
  version: 1.0.0
  title: x-order
components:
  schemas:
    Client:
      type: object
      required:
        - name
      properties:
        a_name:
          type: string
        id:
          type: number
    ClientWithExtension:
      type: object
      required:
        - name
      properties:
        a_name:
          type: string
          x-order: 2
        id:
          type: number
          x-order: 1
```

From here, we now get two different forms of the same type definition.

```go
// Client defines model for Client.
type Client struct {
	AName *string  `json:"a_name,omitempty"`
	Id    *float32 `json:"id,omitempty"`
}

// ClientWithExtension defines model for ClientWithExtension.
type ClientWithExtension struct {
	Id    *float32 `json:"id,omitempty"`
	AName *string  `json:"a_name,omitempty"`
}
```

You can see this in more detail in [the example code](examples/extensions/xorder/).

</details>
</td>
</tr>

<tr>
<td>

`x-oapi-codegen-only-honour-go-name`

</td>
<td>
Only honour the `x-go-name` when generating field names
</td>
<td>
<details>

> [!WARNING]
> Using this option may lead to cases where `oapi-codegen`'s rewriting of field names to prevent clashes with other types, or to prevent including characters that may not be valid Go field names.

In some cases, you may not want use the inbuilt options for converting an OpenAPI field name to a Go field name, such as the `name-normalizer: "ToCamelCaseWithInitialisms"`, and instead trust the name that you've defined for the type better.

In this case, you can use `x-oapi-codegen-only-honour-go-name` to enforce this, alongside specifying the `allow-unexported-struct-field-names` compatibility option.

This allows you to take a spec such as:

```yaml
openapi: "3.0.0"
info:
  version: 1.0.0
  title: x-oapi-codegen-only-honour-go-name
components:
  schemas:
    TypeWithUnexportedField:
      description: A struct will be output where one of the fields is not exported
      properties:
        name:
          type: string
        id:
          type: string
          # NOTE that there is an explicit usage of a lowercase character
          x-go-name: accountIdentifier
          x-oapi-codegen-extra-tags:
            json: "-"
          x-oapi-codegen-only-honour-go-name: true
```

And we'll generate:

```go
// TypeWithUnexportedField A struct will be output where one of the fields is not exported
type TypeWithUnexportedField struct {
	accountIdentifier *string `json:"-"`
	Name              *string `json:"name,omitempty"`
}
```

You can see this in more detail in [the example code](examples/extensions/xoapicodegenonlyhonourgoname).

</details>
</td>
</tr>

</table>

## Custom code generation

It is possible to extend the inbuilt code generation from `oapi-codegen` using Go's `text/template`s.

You can specify, through your configuration file, the `output-options.user-templates` setting to override the inbuilt templates and use a user-defined template.

> [!NOTE]
> Filenames given to the `user-templates` configuration must **exactly** match the filename that `oapi-codegen` is looking for

### Local paths

Within your configuration file, you can specify relative or absolute paths to a file to reference for the template, such as:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/oapi-codegen/oapi-codegen/HEAD/configuration-schema.json
# ...
output-options:
  user-templates:
    client-with-responses.tmpl: ./custom-template.tmpl
    additional-properties.tmpl: /tmp/foo.bar
    typedef.tmpl: no-prefix.tmpl
```

> [!WARNING]
> We do not interpolate `~` or `$HOME` (or other environment variables) in paths given

### HTTPS paths

It is also possible to use HTTPS URLs.

> [!WARNING]
> Although possible, this does lead to `oapi-codegen` executions not necessarily being reproducible. It's recommended to vendor (copy) the OpenAPI spec into your codebase and reference it locally
>
> See [this blog post](https://www.jvt.me/posts/2024/04/27/github-actions-update-file/) for an example of how to use GitHub Actions to manage the updates of files across repos
>
> This will be disabled by default (but possible to turn back on via configuration) [in the future](https://github.com/oapi-codegen/oapi-codegen/issues/1564)

To use it, you can use the following configuration:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/oapi-codegen/oapi-codegen/HEAD/configuration-schema.json
# ...
output-options:
  user-templates:
    # The following are referencing a version of the default client-with-responses.tmpl file, but loaded in through GitHub's raw.githubusercontent.com. The general form to use raw.githubusercontent.com is as follows https://raw.githubusercontent.com/<username>/<project>/<commitish>/path/to/template/template.tmpl

    # Alternatively using raw.githubusercontent.com with a hash
    client-with-responses.tmpl: https://raw.githubusercontent.com/oapi-codegen/oapi-codegen/ad5eada4f3ccc28a88477cef62ea21c17fc8aa01/pkg/codegen/templates/client-with-responses.tmpl
    # Alternatively using raw.githubusercontent.com with a tag
    client-with-responses.tmpl: https://raw.githubusercontent.com/oapi-codegen/oapi-codegen/v2.1.0/pkg/codegen/templates/client-with-responses.tmpl
    # Alternatively using raw.githubusercontent.com with a branch
    client-with-responses.tmpl: https://raw.githubusercontent.com/oapi-codegen/oapi-codegen/master/pkg/codegen/templates/client-with-responses.tmpl
```

> [!WARNING]
> If using URLs that pull locations from a Git repo, such as `raw.githubusercontent.com`, it is strongly encouraged to use a tag or a raw commit hash instead of a branch like `main`. Tracking a branch can lead to unexpected API drift, and loss of the ability to reproduce a build.

### Inline template

It's also possible to set the templates inline in the configuration file:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/oapi-codegen/oapi-codegen/HEAD/configuration-schema.json
# ...
output-options:
  user-templates:
    # NOTE the use of the `|` (pipe symbol) here to denote that this is a
    # multi-line statement that should preserve newlines. More reading:
    # https://stackoverflow.com/a/18708156/2257038 and
    # https://stackoverflow.com/a/15365296/2257038
    client-with-responses.tmpl: |
        // ClientWithResponses builds on ClientInterface to offer response payloads
        type ClientWithResponses struct {
            ClientInterface
        }
        ...
```

### Using the Go package

Alternatively, you are able to use the underlying code generation as a package, which [will be documented in the future](https://github.com/oapi-codegen/oapi-codegen/issues/1487).

## Additional Properties (`additionalProperties`)

[OpenAPI Schemas](https://spec.openapis.org/oas/v3.0.3.html#schema-object) implicitly accept `additionalProperties`, meaning that any fields provided, but not explicitly defined via properties on the schema are accepted as input, and propagated. When unspecified, OpenAPI defines that the `additionalProperties` field is assumed to be `true`.

For simplicity, and to remove a fair bit of duplication and boilerplate, `oapi-codegen` decides to ignore the implicit `additionalProperties: true`, and instead requires you to specify the `additionalProperties` key to generate the boilerplate.

> [!NOTE]
> In the future [this will be possible](https://github.com/oapi-codegen/oapi-codegen/issues/1514) to disable this functionality, and honour the implicit `additionalProperties: true`

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

## Changing the names of generated types

As of `oapi-codegen` v2.2.0, it is now possible to use the `output-options` configuration's `name-normalizer` to define the logic for how to convert an OpenAPI name (i.e. an Operation ID or a Schema name) and construct a Go type name.

<details>

<summary>Example, using default configuration</summary>

By default, `oapi-codegen` will perform camel-case conversion, so for a spec such as:

```yaml
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Example code for the `name-normalizer` output option
paths:
  /api/pets/{petId}:
    get:
      summary: Get pet given identifier.
      operationId: getHttpPet
      parameters:
      - name: petId
        in: path
        required: true
        schema:
          type: string
      responses:
        '200':
          description: valid pet
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Pet'
components:
  schemas:
    Pet:
      type: object
      required:
        - uuid
        - name
      properties:
        uuid:
          type: string
          description: The pet uuid.
        name:
          type: string
          description: The name of the pet.
    Error:
      required:
        - code
        - message
      properties:
        code:
          type: integer
          format: int32
          description: Error code
        message:
          type: string
          description: Error message
    OneOf2things:
      description: "Notice that the `things` is not capitalised"
      oneOf:
        - type: object
          required:
            - id
          properties:
            id:
              type: integer
        - type: object
          required:
            - id
          properties:
            id:
              type: string
              format: uuid
```

This will produce:

```go
// OneOf2things Notice that the `things` is not capitalised
type OneOf2things struct {
	union json.RawMessage
}

// Pet defines model for Pet.
type Pet struct {
	// Name The name of the pet.
	Name string `json:"name"`

	// Uuid The pet uuid.
	Uuid string `json:"uuid"`
}

// The interface specification for the client above.
type ClientInterface interface {
	// GetHttpPet request
	GetHttpPet(ctx context.Context, petId string, reqEditors ...RequestEditorFn) (*http.Response, error)
}
```

</details>

<details>

<summary>Example, using <code>ToCamelCaseWithInitialisms</code></summary>

By default, `oapi-codegen` will perform camel-case conversion, so for a spec such as:

```yaml
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Example code for the `name-normalizer` output option
paths:
  /api/pets/{petId}:
    get:
      summary: Get pet given identifier.
      operationId: getHttpPet
      parameters:
      - name: petId
        in: path
        required: true
        schema:
          type: string
      responses:
        '200':
          description: valid pet
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Pet'
components:
  schemas:
    Pet:
      type: object
      required:
        - uuid
        - name
      properties:
        uuid:
          type: string
          description: The pet uuid.
        name:
          type: string
          description: The name of the pet.
    Error:
      required:
        - code
        - message
      properties:
        code:
          type: integer
          format: int32
          description: Error code
        message:
          type: string
          description: Error message
    OneOf2things:
      description: "Notice that the `things` is not capitalised"
      oneOf:
        - type: object
          required:
            - id
          properties:
            id:
              type: integer
        - type: object
          required:
            - id
          properties:
            id:
              type: string
              format: uuid
```

This will produce:

```go
// OneOf2things Notice that the `things` is not capitalised
type OneOf2things struct {
	union json.RawMessage
}

// Pet defines model for Pet.
type Pet struct {
	// Name The name of the pet.
	Name string `json:"name"`

	// UUID The pet uuid.
	UUID string `json:"uuid"`
}

// The interface specification for the client above.
type ClientInterface interface {
	// GetHTTPPet request
	GetHTTPPet(ctx context.Context, petID string, reqEditors ...RequestEditorFn) (*http.Response, error)
}
```

</details>


For more details of what the resulting code looks like, check out [the test cases](internal/test/outputoptions/name-normalizer/).

## Examples

The [examples directory](examples) contains some additional cases which are useful examples for how to use `oapi-codegen`, including how you'd take the Petstore API and implement it with `oapi-codegen`.

You could also find some cases of how the project can be used by checking out our [internal test cases](internal/test) which are real-world usages that make up our regression tests.

### Blog posts

We love reading posts by the community about how to use the project.

Here are a few we've found around the Web:

- [Building a Go RESTful API with design-first OpenAPI contracts](https://www.jvt.me/posts/2022/07/12/go-openapi-server/)
- [A Practical Guide to Using oapi-codegen in Golang API Development with the Fiber Framework](https://medium.com/@fikihalan/a-practical-guide-to-using-oapi-codegen-in-golang-api-development-with-the-fiber-framework-bce2a59380ae)
- [Generating Go server code from OpenAPI 3 definitions](https://ldej.nl/post/generating-go-from-openapi-3/)
- [Go Client Code Generation from Swagger and OpenAPI](https://medium.com/@kyodo-tech/go-client-code-generation-from-swagger-and-openapi-a0576831836c)
- [Go oapi-codegen + request validation](https://blog.commitsmart.com/go-oapi-codegen-request-validation-285398b37dc8)
- [Streamlining Go + Chi Development: Generating Code from an OpenAPI Spec](https://i4o.dev/blog/oapi-codegen-with-chi-router)

Got one to add? Please raise a PR!

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

If you'd like to reduce what's generated, you can use one of a few options in [the configuration file](#usage) to tune the generation of the resulting output:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/oapi-codegen/oapi-codegen/HEAD/configuration-schema.json
output-options:
  include-tags: []
  exclude-tags: []
  include-operation-ids: []
  exclude-operation-ids: []
  exclude-schemas: []
```

Check [the docs](https://pkg.go.dev/github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen#OutputOptions) for more details of usage.

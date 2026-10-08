# Nullable Union Example

This example demonstrates the optimization for `anyOf` and `oneOf` unions that contain exactly 2 elements where one is `null`.

## Behavior

When a schema uses `anyOf` or `oneOf` with exactly 2 elements and one of them is `type: "null"`, the generator treats it as a nullable property (a pointer) instead of creating a union type.

### Examples

```yaml
# anyOf with string and null
name:
  anyOf:
    - type: string
    - type: "null"
```
Generates: `Name *string` (not a union)

```yaml
# oneOf with integer and null
age:
  oneOf:
    - type: integer
    - type: "null"
```
Generates: `Age *int` (not a union)

```yaml
# anyOf with ref and null
address:
  anyOf:
    - $ref: '#/components/schemas/Address'
    - type: "null"
```
Generates: `Address *Address` (not a union)

```yaml
# oneOf with null first
contact:
  oneOf:
    - type: "null"
    - $ref: '#/components/schemas/Contact'
```
Generates: `Contact *Contact` (not a union)

### Required and nullable

```yaml
# required, but may be null
Event:
  type: object
  required:
    - updatedAt
  properties:
    updatedAt:
      anyOf:
        - type: string
          format: date-time
        - type: "null"
```
Generates: ``UpdatedAt *time.Time `json:"updatedAt"` ``

Nullable is not optional: the key has to be there, so it gets no `omitempty` and a nil pointer encodes as `null`. An optional property still gets `omitempty` and is left out when nil.

## Benefits

1. **Simpler types**: No need for union wrapper types when a field may be null
2. **Better ergonomics**: Direct access to the value without unwrapping from a union
3. **Cleaner validation**: Standard nullable field validation instead of union validation
4. **Consistent with OpenAPI semantics**: `anyOf[T, null]` means a T or null, the same as `type: [T, "null"]`

## When Union Types Are Still Created

Union types are still created when:
- There are more than 2 elements (e.g., `anyOf: [string, integer, null]`)
- There are 2 non-null elements (e.g., `anyOf: [string, integer]`)
- Combined with other combinators like `allOf`


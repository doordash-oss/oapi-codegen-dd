# Union Query Parameters Example

This example demonstrates parameters whose schema is a union with two or more non-null variants.

## Behavior

A parameter union is generated as a named type with its own marshaler, so the value reaches the request. A single non-null variant next to `null` still collapses to a plain pointer.

```yaml
# two non-null variants and null
- name: parentID
  in: query
  schema:
    anyOf:
      - $ref: '#/components/schemas/WidgetID'
      - type: string
        enum: ["null"]
      - type: "null"
```
Generates: `ParentID *ListGadgetsQuery_ParentID`, which wraps the `anyOf` and encodes as `parentID=w1`.

```yaml
# one non-null variant and null
- name: order
  in: query
  schema:
    anyOf:
      - type: string
        enum: ["asc", "desc"]
      - type: "null"
```
Generates: `Order *ListGizmosQueryOrderAnyOf` (no wrapper).

The same applies to path and header parameters.

The config also generates a `std-http` handler, so the request adapter has to parse into the same named type; the example would not compile otherwise.

---
aliases:
- ../../configuration-language/standard-library/trim_space/
canonical: https://grafana.com/docs/agent/latest/flow/reference/stdlib/trim_space/
title: trim_space
---

# trim_space

`trim_space` removes any whitespace characters from the start and end of a string.

## Examples

```river
> trim_space("  hello\n\n")
"hello"
```

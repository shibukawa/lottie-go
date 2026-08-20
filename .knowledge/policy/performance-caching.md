---
id: policy:performance-caching
type: policy
title: Performance Caching Policy
---

Mitigations for CPU triangulation cost (metric:performance-targets):

```yaml
mitigations:
  - cache vertices and indices for geometry-static paths
  - carry static flag on property tracks to detect non-animating paths
  - use Path.Reset() to suppress allocations
  - pool vector.Path instances instead of per-frame allocation
```

Continue the project.

Implement the Semantic Layer.

Goal:

The agent understands business concepts, and does not interpret Elasticsearch fields directly.

Directory:

internal/semantic/

Implement:

Metric
Dimension
Entity
DataSource
Query
Formula

Examples:

Metric:

ROAS
CTR
CPC
CPM
CVR
CPA
Spend
Revenue
Conversions

Dimension:

Channel
Campaign
AdGroup
Creative
Country
Device
Date

Implement:

SemanticRegistry

Support:

RegisterMetric
GetMetric
ListMetrics
RegisterDimension
GetDimension
ListDimensions

Load YAML from:

configs/semantic/

Example:

roas:
  name: ROAS
  description: Return on Advertising Spend
  formula: revenue / spend

ctr:
  name: CTR
  formula: clicks / impressions

Requirements:

Agent input:

"Why did Meta ROI decline yesterday"

must parse to:

metric = ROAS
channel = Meta
date = yesterday

then produce:

SemanticQuery

containing:

metrics
dimensions
filters
time_range
order_by
limit

Implement:

the Semantic Query Compiler.

SemanticQuery
→ Elasticsearch Query DSL

Note:

The agent must not generate Elasticsearch DSL directly.

The path must be:

Agent
→ Semantic Query
→ Compiler
→ ES DSL

Add a Query Validator.

Limits:

- allowed metrics
- allowed dimensions
- allowed filters
- max time range
- max result size

Add tests:

ROAS
CTR
Campaign
Creative
Date Range
Filter
Aggregation

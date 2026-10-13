Continue development.

Implement the Memory system.

Directory:

internal/memory/

Implement three memory layers:

1. Short Term Memory
2. Task Memory
3. Long Term Memory

Interface:

MemoryStore:

Save
Get
Search
Delete

Short Term stores:

Goal
Plan
Decision
ToolCall
Observation

Task Memory stores:

Task
Conclusion
Evidence
Failures
SuccessfulActions

Long Term stores:

UserPreference
BusinessKnowledge
HistoricalInsight
QueryPattern

Phase 1 uses PostgreSQL.

Create tables:

memory
task_memory
long_term_memory

Requirements:

1. Every agent run has a run_id.
2. Memory is associated with run_id.
3. Support TTL.
4. Support pagination.
5. Support metadata.
6. Do not store API keys.
7. Do not store sensitive credentials.
8. A memory write failure must not crash the main agent flow.

Add:

MemoryTool

The agent may call:

search_memory
save_memory

Implement tests.

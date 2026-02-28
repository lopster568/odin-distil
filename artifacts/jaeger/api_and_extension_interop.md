# API and Extension Interoperability

## GraphQL Implementation
The Jaeger query service uses `gqlgen` for its GraphQL implementation.
- **Schema**: `jaeger/query/graphql/schema.graphql`
- **Resolver**: `jaeger/query/graphql/resolver.go`
- **Server**: `jaeger/query/graphql/server.go`

### Extending the Schema for AI Skills
To implement Phase 2, we need to add new types and queries to the GraphQL schema. Since `gqlgen` generates code based on the schema, this involves:
1.  Modifying `schema.graphql` to include `Skill` and `Analysis` types.
2.  Updating `resolver.go` to handle these new fields.
3.  Connecting the `Resolver` to the new `SkillsEngine` (likely via an interface).

## OTel Extension Dependency Graph
Jaeger v2 uses OTel extensions to wrap these services.
- `jaeger_storage`: Provides the storage backend.
- `jaeger_query`: Depends on `jaeger_storage`. Wraps the GraphQL/HTTP query service.
- `jaeger_mcp`: Depends on `jaeger_query`. Provides the MCP server for LLMs.
- **Proposed `ai_skills`**: Should depend on both `jaeger_query` (to contribute to the UI/API) and `jaeger_mcp` (to use its tools).

### Challenge: Circular Dependencies or Tight Coupling
If `ai_skills` needs to add resolvers to `jaeger_query`, but `jaeger_query` is a separate extension, we need a clean way to "plug in" new resolvers.
- **Solution**: The `jaeger_query` extension could accept a list of "Extra Resolvers" or "Plugins" during its initialization. We should check if `jaeger/query/graphql` already supports this.

## Frontend-Backend Contract
The Jaeger UI (`Assistant` component) currently uses Apollo Client to fetch traces.
- **Current Query**: `GET_TRACE` in `jaeger/cmd/jaeger-ui/src/graphql/queries.ts`.
- **Target Query**: A new `ANALYZE_TRACE` query that takes a `skillId` and returns the AI's response along with its reasoning steps.

## Model Context Protocol (MCP) as a Toolset
The `jaegermcp` extension is already functional and provides a set of tools (Search, GetTrace, etc.). The `ai_skills` engine will utilize these tools.
- Instead of re-implementing trace fetching logic, the `ai_skills` extension can instantiate a `langchaingo` agent and provide it with tools that call the `jaegermcp` handlers.

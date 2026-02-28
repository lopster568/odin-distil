# Frontend Integration and AI Assistant UI

## Jaeger UI (React) Structure
The Jaeger UI is located in `jaeger/cmd/jaeger-ui/`. It is a React application utilizing:
- **Apollo Client/GraphQL**: For data fetching (seen in `Assistant/index.tsx`).
- **Styled Components**: For styling.
- **TypeScript**: For type safety.

### AI-Related Components
- **`Assistant` Component**: Located at `jaeger/cmd/jaeger-ui/src/components/Assistant/`. Currently, it appears to be a basic implementation that queries a trace and displays its JSON. This is the entry point for the AI assistant.
- **`CriticalPath` Component**: Located at `jaeger/cmd/jaeger-ui/src/components/TracePage/CriticalPath/`. This aligns with one of the requested skills ("Analyze Critical Path"). Phase 2 should integrate the AI's "reasoning" with this visual component.

## Backend Communication
The UI uses GraphQL (`@apollo/client`) to communicate with the backend. This implies:
1.  The `jaeger_query` extension likely exposes a GraphQL endpoint.
2.  The new "Skills Engine" will need to expose its functionality via this GraphQL schema or a compatible HTTP endpoint that the UI can consume.

## Phase 2 UI Enhancements
To support the "Self-Service Skills" framework, the UI needs to be updated to:
1.  **Skill Discovery**: Fetch the list of available skills (loaded from YAML on the backend) and display them in the `Assistant` interface.
2.  **Reasoning Visualization**: Display the "reasoning steps" (Chain-of-Thought) taken by the LangChainGo agent.
3.  **Contextual Actions**: Add buttons or triggers within the `TracePage` to invoke specific skills (e.g., "Analyze this trace's critical path").

## Proposed API Extension
Since the UI uses GraphQL, the `aiskills` extension should ideally contribute to the GraphQL schema:
```graphql
type Skill {
  id: String!
  description: String!
}

type AnalysisResult {
  text: String!
  steps: [String]
}

extend type Query {
  listSkills: [Skill!]!
  analyzeTrace(skillId: String!, traceId: String!, query: String): AnalysisResult!
}
```

## Risks and Gaps
- **GraphQL Schema Extensibility**: I need to verify how the `jaeger_query` extension handles GraphQL schema generation and if it's easy to "extend" from another extension (`aiskills`).
- **Real-time Feedback**: AI reasoning can take time. The current GraphQL setup might need to support Subscriptions or the UI will need to handle long-polling/loading states gracefully.

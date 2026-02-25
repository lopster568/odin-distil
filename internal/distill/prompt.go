package distill

import "fmt"

// responsibilityPrompt builds the Stage 2 prompt for a single bucket.
// chunks is the concatenated text context for that bucket.
func responsibilityPrompt(bucket, chunksContext string) string {
	return fmt.Sprintf(`You are an expert Kubernetes and distributed systems architect.

Analyze the following source code excerpts from the Kubernetes "%s" subsystem.
Return ONLY a single valid JSON object — no explanation, no markdown fences, no commentary.

The JSON must match this exact schema:
{
  "component": "<bucket name>",
  "core_responsibility": "<one sentence describing the primary job>",
  "state_managed": ["<state item 1>", "..."],
  "event_sources": ["<event or trigger source 1>", "..."],
  "reconciliation_logic": "<description of how desired vs actual state is reconciled>",
  "failure_handling": "<description of retry, backoff, error propagation strategy>"
}

=== SOURCE EXCERPTS ===
%s
=== END EXCERPTS ===

Respond with only the JSON object.`, bucket, chunksContext)
}

// patternPrompt builds the Stage 3 prompt fed with all responsibilities JSON.
func patternPrompt(responsibilitiesJSON string) string {
	return fmt.Sprintf(`You are an expert distributed systems architect analyzing the Kubernetes codebase.

Given the following structured responsibility descriptions for each Kubernetes subsystem,
extract the recurring architectural patterns that appear across multiple components.

Return ONLY a valid JSON array — no explanation, no markdown fences.

Each element must match:
{
  "name": "<pattern name>",
  "description": "<what the pattern is and why it exists>",
  "examples": ["<subsystem or file where this pattern appears>", "..."]
}

=== SUBSYSTEM RESPONSIBILITIES ===
%s
=== END ===

Respond with only the JSON array.`, responsibilitiesJSON)
}

// controlLoopPrompt builds the Stage 4 prompt targeting the controllers bucket.
func controlLoopPrompt(controllerResponsibility string) string {
	return fmt.Sprintf(`You are an expert Kubernetes contributor explaining the reconciliation loop pattern.

Based on this responsibility description for the Kubernetes controllers subsystem:

%s

Write a detailed markdown document that covers:
1. A generalized pseudocode template of the reconciliation loop
2. Idempotency guarantees — how controllers handle repeated reconciliation safely
3. Retry and failure semantics — backoff strategy, error propagation
4. Where workqueue and informer patterns are used and why
5. The relationship between desired state (spec) and observed state (status)

Use markdown headings, code blocks for pseudocode, and be precise and technical.
Target audience: a senior engineer new to Kubernetes internals.`, controllerResponsibility)
}

// frictionPrompt builds the Stage 5 prompt using patterns and responsibilities.
func frictionPrompt(responsibilitiesJSON, patternsJSON string) string {
	return fmt.Sprintf(`You are a senior Kubernetes contributor reviewing the codebase for contributor experience.

Given the following architectural data:

=== SUBSYSTEM RESPONSIBILITIES ===
%s
=== END ===

=== ARCHITECTURAL PATTERNS ===
%s
=== END ===

Identify areas of high cognitive friction, contributor pain points, and GSoC-suitable opportunities.

Return ONLY a valid JSON array — no explanation, no markdown fences.

Each element must match:
{
  "area": "<subsystem or mechanism name>",
  "friction_type": "<complexity | coupling | missing abstraction | poor observability | other>",
  "rationale": "<why this area is difficult or painful>",
  "opportunity": "<concrete improvement or GSoC project idea>"
}

Respond with only the JSON array.`, responsibilitiesJSON, patternsJSON)
}

// studyTopicsPrompt builds the Stage 6 prompt using all prior artifacts.
func studyTopicsPrompt(responsibilitiesJSON, patternsJSON, frictionJSON, controlLoopsMD string) string {
	return fmt.Sprintf(`You are a distributed systems educator building a study roadmap for a Kubernetes contributor.

Given the following architectural intelligence extracted from the Kubernetes codebase:

=== SUBSYSTEM RESPONSIBILITIES ===
%s
=== END ===

=== ARCHITECTURAL PATTERNS ===
%s
=== END ===

=== FRICTION AREAS ===
%s
=== END ===

=== RECONCILIATION LOOP ANALYSIS ===
%s
=== END ===

Generate 25-30 deep study topics. Each topic must:
- Reference a real Kubernetes mechanism or concept
- Generalize to a broader distributed systems principle
- Be relevant to system design interviews or GSoC proposals

Format as markdown. For each topic use this structure:

### <Topic Title>
**Kubernetes mechanism:** <specific file path or API if known>
**Distributed systems principle:** <the broader concept>
**Why it matters:** <one sentence rationale>

Produce all 25-30 topics.`, responsibilitiesJSON, patternsJSON, frictionJSON, controlLoopsMD)
}

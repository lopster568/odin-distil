Jaeger
AI-Powered Trace Analysis: Phase 2 - Self-Service "Skills" Framework
* Description: Jaeger is the industry-standard platform for distributed tracing. As microservice architectures grow complex, finding root causes in massive trace data becomes increasingly difficult. While Phase 1 of this initiative established a baseline AI assistant for natural language search, the system currently relies on hard-coded capabilities. This project (Phase 2) aims to transform the Jaeger AI agent from a static chatbot into an extensible, user-programmable platform. The primary objective is to implement a "Self-Service Skills" framework, architecturally similar to "Claude Code Skills." This will allow end-users to teach the Jaeger AI new debugging workflows (e.g., "Analyze Critical Path" or "Detect N+1 Queries") by simply adding configuration files containing system prompts and logic rules, without needing to recompile the Jaeger binary. The applicant will build this extension within the Jaeger v2 (OpenTelemetry-based) architecture, utilizing LangChainGo to orchestrate interactions with Language Models (SLMs/LLMs). This project bridges the gap between generic AI reasoning and domain-specific observability expertise.
* Expected Outcome:
   * Skills Engine Implementation: A robust backend framework in Go that dynamically discovers, validates, and loads user-defined "Skills" (prompts and tool definitions) from configuration.
   * Smart Analysis Features: A polished implementation of Natural Language Search and Contextual Trace Explanation that intelligently leverages these loaded skills.
   * Local-First Support: Verified compatibility with local model runners (e.g., Ollama, Llama.cpp) to ensure deterministic performance without sending data to public clouds.
   * UI Integration: Enhancements to the Jaeger React UI to expose these AI capabilities and visualize the "reasoning steps" taken by the agent.
   * Documentation: A complete guide for users on "How to Author Custom AI Skills for Jaeger."
* Learning Opportunities:
   * Agentic AI Architecture: Learn to design stateful AI agents in Go that utilize "Tool Calling" and "Reasoning Loops" rather than simple text generation.
   * OpenTelemetry Internals: Gain deep familiarity with the OpenTelemetry Collector architecture, as Jaeger v2 is built directly on top of it.
   * Cloud-Native Engineering: Experience contributing to a graduated CNCF project, including navigating code reviews, writing design docs (RFDs), and adhering to open-source best practices.
   * Full-Stack Development: Practical experience bridging a complex Go backend with a modern React frontend.
* Recommended Skills:
   * Languages: Strong proficiency in Go (Golang) is required. Experience with TypeScript/React is highly recommended.
   * AI/LLM: Familiarity with LLM concepts (Prompt Engineering, RAG, Function Calling) and frameworks like LangChain.
   * Domain Knowledge: Basic understanding of distributed systems, observability, or debugging workflows is beneficial.
* Expected project size: Large (~350 hour projects)
* Mentors:
   * Jonah Kowall (@jkowall, __jkowall@kowall.net__)
   * Yuri Shkuro (@yurishkuro, __github@ysh.us__)
* Upstream Issue: __jaegertracing/jaeger#7827__

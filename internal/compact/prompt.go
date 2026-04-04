// Package compact implements the three-tier conversation compaction system
// for ClawGo: auto-compaction (threshold-based), micro-compaction (tool result
// summarization), and reactive compaction (prompt-too-long recovery).
// This mirrors the TypeScript services/compact/ implementation.
package compact

// BaseCompactPrompt is the system prompt used when compacting the full conversation.
// It instructs Claude to produce a structured summary preserving key technical
// details, decisions, and pending work. Matches the TS services/compact/prompt.ts.
const BaseCompactPrompt = `You are a conversation summarizer. Your task is to create a detailed summary of the conversation so far, preserving all important technical details, decisions, and context needed to continue the work.

Please analyze the conversation and create a summary with the following sections:

1. **Primary Request**: What the user originally asked for and any refinements to the request
2. **Key Technical Concepts**: Important technical details, patterns, and approaches discussed
3. **Files and Code Sections**: Specific files, functions, code snippets, and their purposes that were discussed or modified
4. **Errors and Fixes**: Any errors encountered and how they were resolved
5. **Problem Solving**: The reasoning and approach taken to solve problems
6. **All User Messages**: Key points from all user messages (preserve exact requirements and preferences)
7. **Pending Tasks**: Any incomplete work or next steps that were discussed
8. **Current Work**: What was being worked on most recently, with full context needed to continue

Format your response as follows:

<analysis>
[Your internal analysis of the conversation - this will be stripped from the final output]
</analysis>

<summary>
[Your detailed summary following the sections above]
</summary>

Important guidelines:
- Preserve ALL technical details (file paths, function names, variable names, error messages)
- Include exact code snippets when they are important for context
- Maintain the chronological flow of problem-solving
- Do not omit any user requirements or preferences
- Be thorough rather than brief - context preservation is more important than conciseness`

// PartialCompactPrompt is used for micro-compaction of individual tool results.
// It produces a shorter summary focused on the key output rather than the full
// conversation context.
const PartialCompactPrompt = `Summarize the following tool output concisely, preserving key information:
- File paths and line numbers mentioned
- Error messages and their causes
- Key data points and results
- Any actionable information

Keep the summary under 200 words. Focus on information needed to continue the conversation.`

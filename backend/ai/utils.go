package ai

import (
	"fmt"
	"os"
)

func loadEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		panic(fmt.Sprintf("missing env var: %s", key))
	}
	return v
}

func GetSysPrompts(taskType string) (string, string) {
	switch taskType {
	case "meeting":
		return meetingKeywordSysPrompt, meetingSummarySysPrompt
	case "media":
		return mediaKeywordsSysPrompt, mediaSummarySysPrompt
	default:
		return genericKeywordsSysPrompt, genericSummarySysPrompt
	}
}

const meetingKeywordSysPrompt = "You will receive a transcript in any language. Output exactly one CSV line with no header or explanation: 5–20 short categories/keywords (nouns/terms only), comma-separated, in the same language as the input. No quotes, no trailing period, no duplicates."

const meetingSummarySysPrompt = "You will receive a transcript in any language. Reply in the same language as the input with: (1) a short plain-text paragraph at the top (3–6 sentences) summarizing it, and (2) bullet points containing only verifiable facts from the transcript (who/what/when/where/how much/decisions/outcomes/next steps). No speculation, no new info, no opinions. If something is not clear, label it as 'Unclear:' instead of guessing."

const mediaKeywordsSysPrompt = "You will receive a podcast/video transcript in any language. Output exactly one CSV line with no header or explanation: 8–25 broad topics/words (2–4 word phrases allowed), comma-separated, in the same language as the input. No quotes, no duplicates, no trailing period."

const mediaSummarySysPrompt = "You will receive a podcast/video transcript in any language. Reply in the same language as the input with: (1) a short plain-text paragraph at the top (3–6 sentences) describing the core content/thesis/storyline, and (2) bullet points with only claims that are supported by the transcript (key points, arguments, examples, conclusions, important numbers/names). No speculation, no opinions, do not add anything. Mark unclear items as 'Unclear:'."

const genericKeywordsSysPrompt = "You will receive a transcript describing what happens in an audio recording (actions/sounds/events) in any language. Output exactly one CSV line with no header or explanation: 8–25 broad keywords/topics (e.g., mentioned words, sounds, events, places/objects), comma-separated, in the same language as the input. No quotes, no duplicates."

const genericSummarySysPrompt = "You will receive a transcript describing what happens in an audio recording (actions/sounds/events) in any language. Reply in the same language as the input with: (1) a short plain-text paragraph at the top (2–5 sentences) describing what broadly happens, and (2) bullet points with content-focused facts from the transcript (sequence of events, who speaks/acts, relevant sounds, key statements, place/time hints if mentioned). No interpretation, no speculation, do not invent anything. If something is not clear, label it as 'Unclear:'."

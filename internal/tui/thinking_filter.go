package tui

import "strings"

// thinkingFilter separates <think>...</think> blocks from the rest of the content
// as a streaming model emits it chunk by chunk.
//
// Background:
//
//	Models such as GLM-4.7, DeepSeek-R1 and QwQ emit a "chain-of-thought" block
//	wrapped in <think>...</think> tags before their actual answer.  This internal
//	reasoning is useful to show to the user, but it must NOT be stored in the
//	conversation history that is sent back to the API on subsequent turns:
//	  - The model does not expect to receive its own thinking back.
//	  - Sending it back can confuse the model and inflates the token count.
//	  - Some providers (e.g. GLM-4.7) may reject messages that contain <think> tags.
//	OpenAI o1/o3 handle this cleanly by placing reasoning in a separate
//	`reasoning_content` field; for OpenAI-compat providers we must do it ourselves.
//
// Strategy (option B):
//   - Display: show everything, including <think> blocks (styled differently).
//   - History:  strip <think>…</think> blocks; only store the "answer" part.
//
// Chunk-boundary safety:
//
//	Tags may be split across multiple streaming chunks, e.g. one chunk ends with
//	"<thi" and the next begins with "nk>".  thinkingFilter keeps a small carry
//	buffer so that a partial tag at the end of a chunk is not prematurely written
//	to history, but is held until the next chunk resolves the ambiguity.
type thinkingFilter struct {
	// inThinking is true while we are inside a <think>…</think> block.
	// Content received while inThinking is true is withheld from historyBuf.
	inThinking bool

	// carry holds bytes from the end of the previous chunk that could be the
	// start of a tag but have not yet been confirmed.  For example, if a chunk
	// ends with "<thi", carry = "<thi" until we see whether the next chunk
	// continues with "nk>" (opening tag) or something else (not a tag).
	// The maximum carry length is len("</think>") - 1 = 7 bytes.
	carry string
}

const (
	thinkOpen  = "<think>"
	thinkClose = "</think>"

	// maxCarry is the longest prefix we might need to buffer: len(thinkClose)-1.
	maxCarry = len(thinkClose) - 1
)

// Feed processes one streaming chunk and returns two strings:
//
//   - displayPart: text to append to the TUI viewport (includes thinking blocks).
//   - historyPart: text to append to the message history (thinking blocks stripped).
//
// The caller should call Flush() after the stream ends to drain any remaining carry.
func (f *thinkingFilter) Feed(chunk string) (displayPart, historyPart string) {
	// Prepend any unresolved carry from the previous chunk.
	// We work on the combined string so that tags split across chunks are handled.
	s := f.carry + chunk
	f.carry = ""

	var display strings.Builder
	var history strings.Builder

	for len(s) > 0 {
		if f.inThinking {
			// Look for the closing tag.
			if idx := strings.Index(s, thinkClose); idx >= 0 {
				// Found the closing tag.
				// Everything up to and including </think> belongs to display only.
				display.WriteString(s[:idx+len(thinkClose)])
				s = s[idx+len(thinkClose):]
				f.inThinking = false
			} else {
				// No closing tag in this chunk.  We might be sitting on a partial
				// tag at the very end (e.g. s ends with "</thi").  Keep up to
				// maxCarry bytes in carry so the next Feed() can complete the tag.
				if len(s) > maxCarry {
					display.WriteString(s[:len(s)-maxCarry])
					f.carry = s[len(s)-maxCarry:]
				} else {
					f.carry = s
				}
				s = ""
			}
		} else {
			// Look for the opening tag.
			if idx := strings.Index(s, thinkOpen); idx >= 0 {
				// Everything before <think> goes to both display and history.
				display.WriteString(s[:idx])
				history.WriteString(s[:idx])
				// The <think> tag itself and onwards goes to display only.
				display.WriteString(s[idx : idx+len(thinkOpen)])
				s = s[idx+len(thinkOpen):]
				f.inThinking = true
			} else {
				// No opening tag found.  A partial tag might lurk at the end,
				// e.g. the chunk ends with "<thi".  Keep the tail in carry.
				if len(s) > maxCarry {
					safe := s[:len(s)-maxCarry]
					display.WriteString(safe)
					history.WriteString(safe)
					f.carry = s[len(s)-maxCarry:]
				} else {
					// The entire remainder is short enough to be a partial tag.
					f.carry = s
				}
				s = ""
			}
		}
	}

	return display.String(), history.String()
}

// Flush drains the carry buffer at the end of a stream.
// Any bytes still in carry cannot be part of a tag (the stream ended without
// completing one), so they are emitted verbatim to both display and history
// unless we are still inside an unclosed <think> block.
func (f *thinkingFilter) Flush() (displayPart, historyPart string) {
	if f.carry == "" {
		return "", ""
	}
	s := f.carry
	f.carry = ""
	if f.inThinking {
		// Still inside a think block at EOF — treat as display-only.
		return s, ""
	}
	// Not inside a think block: the carry is plain content.
	return s, s
}

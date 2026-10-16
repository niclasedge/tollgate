// Package meter reads the token usage the upstream reports (non-streaming
// JSON and SSE streams) and prices it per model. The pre-request estimate
// lives in the pipeline (pipeline.EstimateTokens).
package meter

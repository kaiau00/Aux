package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aux-ai/aux-cli/internal/config"
	"github.com/aux-ai/aux-cli/internal/llm/models"
	"github.com/aux-ai/aux-cli/internal/logging"
)

func GetAgentPrompt(agentName config.AgentName, provider models.ModelProvider) string {
	basePrompt := ""
	switch agentName {
	case config.AgentCoder:
		basePrompt = CoderPrompt(provider)
	case config.AgentTitle:
		basePrompt = TitlePrompt(provider)
	case config.AgentTask:
		basePrompt = TaskPrompt(provider)
	case config.AgentSummarizer:
		basePrompt = SummarizerPrompt(provider)
	default:
		basePrompt = "You are a helpful assistant"
	}

	// Ponytail Protocol and project-specific context files are scoped to the
	// agents that actually write code. Title and Summarizer must remain
	// unopinionated to keep session titles and summaries clean.
	if !agentUsesPonytail(agentName) {
		return basePrompt
	}

	if contextContent := getContextFromPaths(); contextContent != "" {
		logging.Debug("Context content", "Context", contextContent)
		return appendPonytailProtocol(fmt.Sprintf("%s\n\n# Project-Specific Context\n Make sure to follow the instructions in the context below\n%s", basePrompt, contextContent))
	}
	return appendPonytailProtocol(basePrompt)
}

// agentUsesPonytail reports whether the Ponytail Protocol should be appended
// to the given agent's prompt. Only Coder and Task receive it; Title and
// Summarizer stay unopinionated.
func agentUsesPonytail(agentName config.AgentName) bool {
	return agentName == config.AgentCoder || agentName == config.AgentTask
}

const ponytailProtocol = `### EXECUTION MANDATE: THE PONYTAIL PROTOCOL

Before generating or modifying code, evaluate this YAGNI decision ladder in order:

1. Does this feature actually need to exist? If no, reject the request.
2. Is this logic already implemented elsewhere in the codebase? If yes, reuse it; do not rewrite it.
3. Can the standard language library handle this without adding an external dependency? If yes, use the stdlib exclusively.
4. Does a native platform feature cover this? If yes, use the native feature.
5. If code must be written, write the absolute minimum number of lines required to fulfill the logic. Over-engineering will be penalized.

When skipping an implementation or choosing simpler logic, prepend the relevant code or response with a brief ponytail comment explaining the lazy architectural choice, for example:
# ponytail: stdlib provides this function natively.`

func appendPonytailProtocol(prompt string) string {
	return fmt.Sprintf("%s\n\n%s", prompt, ponytailProtocol)
}

var (
	onceContext    sync.Once
	contextContent string
)

func getContextFromPaths() string {
	onceContext.Do(func() {
		var (
			cfg          = config.Get()
			workDir      = cfg.WorkingDir
			contextPaths = cfg.ContextPaths
		)

		contextContent = processContextPaths(workDir, contextPaths)
	})

	return contextContent
}

func processContextPaths(workDir string, paths []string) string {
	var (
		wg       sync.WaitGroup
		resultCh = make(chan string)
	)

	// Track processed files to avoid duplicates
	processedFiles := make(map[string]bool)
	var processedMutex sync.Mutex

	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()

			if strings.HasSuffix(p, "/") {
				filepath.WalkDir(filepath.Join(workDir, p), func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !d.IsDir() {
						// Check if we've already processed this file (case-insensitive)
						processedMutex.Lock()
						lowerPath := strings.ToLower(path)
						if !processedFiles[lowerPath] {
							processedFiles[lowerPath] = true
							processedMutex.Unlock()

							if result := processFile(path); result != "" {
								resultCh <- result
							}
						} else {
							processedMutex.Unlock()
						}
					}
					return nil
				})
			} else {
				fullPath := filepath.Join(workDir, p)

				// Check if we've already processed this file (case-insensitive)
				processedMutex.Lock()
				lowerPath := strings.ToLower(fullPath)
				if !processedFiles[lowerPath] {
					processedFiles[lowerPath] = true
					processedMutex.Unlock()

					result := processFile(fullPath)
					if result != "" {
						resultCh <- result
					}
				} else {
					processedMutex.Unlock()
				}
			}
		}(path)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	results := make([]string, 0)
	for result := range resultCh {
		results = append(results, result)
	}

	return strings.Join(results, "\n")
}

func processFile(filePath string) string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return "# From:" + filePath + "\n" + string(content)
}

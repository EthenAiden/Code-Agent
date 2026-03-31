# Intent Classifier

The Intent Classifier is the first agent in the Sequential Agent chain. It categorizes user queries into one of three intent types to route them to the appropriate workflow.

## Intent Types

1. **chat** - Normal conversation, questions, or requests that don't involve code generation or modification
2. **generate_code** - Requests to create, generate, build, or implement new code
3. **modify_code** - Requests to modify, change, update, fix, or refactor existing code

## Architecture

The Intent Classifier uses a two-tier classification approach:

### 1. Keyword-Based Classification (Fast Path)
- Performs fast pattern matching using predefined keywords
- Completes in < 1ms for most queries
- Handles clear-cut cases with high confidence (0.85)

**Generate Code Keywords:**
- create, generate, build, implement, write, develop
- make, construct, scaffold, initialize, setup, add new

**Modify Code Keywords:**
- modify, change, update, fix, refactor, edit
- alter, adjust, improve, optimize, rewrite, revise

### 2. LLM-Based Classification (Fallback)
- Used for ambiguous queries that don't match clear keyword patterns
- Provides more nuanced understanding of user intent
- Returns structured JSON with intent, confidence, and reasoning

## Usage

```go
import (
    "context"
    "github.com/ethen-aiden/code-agent/agent/intent"
    "github.com/ethen-aiden/code-agent/agent/model"
)

// Create a chat model
chatModel, err := model.NewChatModel(ctx, &model.ChatModelConfig{
    APIKey: "your-api-key",
    Model:  "gpt-4",
})

// Create intent classifier
classifier := intent.NewIntentClassifier(chatModel)

// Classify a query
classification, err := classifier.Classify(ctx, "create a REST API for user management")

// Use the classification result
switch classification.Intent {
case intent.IntentChat:
    // Handle as normal conversation
case intent.IntentGenerateCode:
    // Route to code generation workflow
case intent.IntentModifyCode:
    // Route to code modification workflow
}
```

## Classification Result

The `IntentClassification` struct contains:

```go
type IntentClassification struct {
    Intent     IntentType  // The classified intent type
    Confidence float64     // Confidence score (0.0 - 1.0)
    Reasoning  string      // Explanation of the classification
}
```

## Performance

- **Keyword-based classification**: < 1ms
- **LLM-based classification**: Typically 100-500ms depending on model
- **Overall requirement**: < 500ms (Requirement 1.5)

## Requirements Satisfied

- **1.1**: Categorizes queries into exactly one of three types
- **1.2**: Detects generate code keywords
- **1.3**: Detects modify code keywords
- **1.4**: Classifies non-code queries as chat
- **1.5**: Returns classification within 500ms

## Testing

Run tests with:
```bash
go test -v ./agent/intent/...
```

Test coverage includes:
- Keyword-based classification for all intent types
- JSON parsing and validation
- Performance benchmarks
- Edge cases and error handling

package agents

import (
	"context"
	"fmt"
	"google.golang.org/genai"
	"github.com/vishnuprasad2004/argus/internal/config"	
)

// Client wraps the official Google Gemini SDK
// replaces the LangChainGo googleai.GoogleAI
type GeminiClient struct {
	client *genai.Client
	model  string
}

func CreateAgent(cfg *config.Config) (*GeminiClient, error) {
	if cfg.GeminiAPIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY not set")
	}

	// Default fallback model if empty in config
	model := cfg.Model
	if model == "" {
		model = "gemini-2.5-flash"
	}
	
	apiKey := cfg.GeminiAPIKey
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY not set")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &GeminiClient{
		client: client,
		model: model, // fast, free tier, stable
	}, nil
}

// Generate is a simple single-turn call — used by sub-agents
func (g *GeminiClient) Generate(ctx context.Context, prompt string) (string, error) {
	result, err := g.client.Models.GenerateContent(ctx,
		g.model,
		genai.Text(prompt),
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}
	return result.Text(), nil
}

// Chat is a multi-turn call with history — used by orchestrator
func (g *GeminiClient) Chat(ctx context.Context, system string, history []ConversationTurn, userMsg string) (string, error) {
	// build content history
	var contents []*genai.Content

	for _, turn := range history {
		role := genai.RoleUser
		if turn.Role == "assistant" {
			role = genai.RoleModel
		}
		contents = append(contents, &genai.Content{
			Role:  role,
			Parts: []*genai.Part{genai.NewPartFromText(turn.Content)},
		})
	}

	// add current user message
	contents = append(contents, &genai.Content{
		Role:  genai.RoleUser,
		Parts: []*genai.Part{genai.NewPartFromText(userMsg)},
	})

	// system instruction goes on the model config
	cfg := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{genai.NewPartFromText(system)},
		},
	}

	result, err := g.client.Models.GenerateContent(ctx,
		g.model,
		contents,
		cfg,
	)
	if err != nil {
		return "", fmt.Errorf("chat: %w", err)
	}
	return result.Text(), nil
}
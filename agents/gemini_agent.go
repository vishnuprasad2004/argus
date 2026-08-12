package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/vishnuprasad2004/argus/internal/config"
	"google.golang.org/genai"
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


// ChatStream — streaming version, used for the FINAL answer shown to user
// onChunk is called for each token chunk as it arrives
// returns the full assembled response when done
func (g *GeminiClient) ChatStream(
    ctx context.Context,
    system string,
    history []ConversationTurn,
    userMsg string,
    onChunk func(string), // called per chunk — TUI uses this to render tokens
) (string, error) {
    contents, cfg := g.buildContents(system, history, userMsg)

    iter := g.client.Models.GenerateContentStream(ctx, g.model, contents, cfg)

    var full strings.Builder

    for result, err := range iter {
        if err != nil {
            // partial response is still useful — return what we got
            if full.Len() > 0 {
                return full.String(), nil
            }
            return "", fmt.Errorf("stream: %w", err)
        }

        chunk := result.Text()
        if chunk == "" {
            continue
        }

        full.WriteString(chunk)
        onChunk(chunk) // fire each chunk to caller immediately
    }

    return full.String(), nil
}


// buildContents shared between Chat and ChatStream
func (g *GeminiClient) buildContents(
    system string,
    history []ConversationTurn,
    userMsg string,
) ([]*genai.Content, *genai.GenerateContentConfig) {
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

    contents = append(contents, &genai.Content{
        Role:  genai.RoleUser,
        Parts: []*genai.Part{genai.NewPartFromText(userMsg)},
    })

    cfg := &genai.GenerateContentConfig{
        SystemInstruction: &genai.Content{
            Parts: []*genai.Part{genai.NewPartFromText(system)},
        },
    }

    return contents, cfg
}

func CreateAgentWithConfig(cfg *config.Config) (*GeminiClient, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}
	return &GeminiClient{
		client: client,
		model:  cfg.Model,
	}, nil
}
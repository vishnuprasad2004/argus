package agents

import (
	"context"
	"os"
	"fmt"
	"github.com/tmc/langchaingo/llms/googleai"
)


// func CreateAgent ()  {
// 	gotenv.Load();
// 	api_key := os.Getenv("GEMINI_API_KEY")
// 	llm, err := googleai.New(context.Background(), googleai.WithAPIKey(api_key), googleai.WithDefaultModel("gemini-2.5-flash"));
// 	if err != nil {
//     fmt.Printf("Error while loading the LLM model: %v\n", err)
//     return
// 	}



// 	// answer, err := llm.GenerateContent(context.Background(),
// 	// 	[]llms.MessageContent{
// 	// 		llms.TextParts(
// 	// 		llms.ChatMessageTypeHuman,
// 	// 		"Explain Kubernetes CrashLoopBackOff",
// 	// 		),
// 	// 	}, 
// 	// 	llms.WithStreamingFunc(
// 	// 		func(ctx context.Context, chunk []byte) error {
// 	// 			fmt.Print(string(chunk))
// 	// 			return nil
// 	// 		}),
// 	// );
// 	// if err != nil {
//   //   fmt.Printf("Error while fetching the LLM response: %v\n", err)
//   //   return
// 	// }
// 	// fmt.Println("\n\nDone!")
// 	// _ = answer
// 	answer, err := llms.GenerateFromSinglePrompt(
// 	context.Background(),
// 	llm,
// 	"Explain Kubernetes CrashLoopBackOff for CLI tool, so please don't use markdown format",
// )

// if err != nil {
// 	fmt.Printf("Error: %v\n", err)
// 	return
// }

// fmt.Println(answer)

// }


func CreateAgent() (*googleai.GoogleAI, error) {
	api_key := os.Getenv("GEMINI_API_KEY");
	if api_key == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY not set. Please set it in your environment variables.")
	}
	llm, err := googleai.New(context.Background(), googleai.WithAPIKey(api_key), googleai.WithDefaultModel("gemini-2.5-flash-lite"));
	if err != nil {
		return nil, err
	}
	return llm, nil
}
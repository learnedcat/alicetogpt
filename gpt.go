package main

import (
	"context"
	"fmt"
	"os"

	openrouter "github.com/revrost/go-openrouter"
)

func query(ctx context.Context, message string, _ string) (Reply, error) {
	client := openrouter.NewClient(
		os.Getenv("OPENROUTER_API_KEY"),
	)

	req := openrouter.ChatCompletionRequest{
		Model: os.Getenv("OPENROUTER_GPT_NAME"),
		Messages: []openrouter.ChatCompletionMessage{
			openrouter.UserMessage(message),
		},
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		fmt.Printf("Error from gpt: %v", err)
		return Reply{}, err
	}

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content.Text
	}

	return Reply{
		Value: content,
		ID:    resp.ID,
	}, nil
}

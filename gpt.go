package main

import (
	"context"
	"fmt"
	"os"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

func query(ctx context.Context, message string, previousResponseID string) (Reply, error) {
	client := openai.NewClient(
		option.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
	)

	/*
		var agentTools = []responses.ToolUnionParam{
			{OfWebSearch: &responses.WebSearchToolParam{Type: "web_search_preview"}},
		}
	*/

	params := responses.ResponseNewParams{
		Model: openai.ChatModelGPT5Nano,
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String("В каком году создали Chat GPT?")},
		Store: openai.Bool(true),
		//Tools:              agentTools,
	}

	if len(previousResponseID) > 0 {
		params.PreviousResponseID = openai.String(previousResponseID)
	}

	resp, err := client.Responses.New(ctx, params)
	if err != nil {
		fmt.Println("Error: %v", err)
		return Reply{}, err
	}

	fmt.Println(resp.OutputText())

	return Reply{Value: resp.OutputText(), ID: resp.ID}, nil
}

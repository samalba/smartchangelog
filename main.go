// A generated module for Smartchangelog functions
//
// This module has been generated via dagger init and serves as a reference to
// basic module structure as you get started with Dagger.
//
// Two functions have been pre-created. You can modify, delete, or add to them,
// as needed. They demonstrate usage of arguments and return types using simple
// echo and grep commands. The functions can be called from the dagger CLI or
// from one of the SDKs.
//
// The first line in this comment block is a short description line and the
// rest is a long description with more detail on the module's purpose or usage,
// if appropriate. All modules should have a short description.

package main

import (
	"context"
	"dagger/smartchangelog/internal/dagger"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
)

type Smartchangelog struct{}

func baseCtr() *dagger.Container {
	// gitDir := dag.Git("https://github.com/dagger/dagger.git", dagger.GitOpts{
	// 	KeepGitDir: true,
	// }).Head().Tree()
	gitRepository := "https://github.com/dagger/dagger.git"

	return dag.Container().From("alpine:3.20").
		WithExec([]string{"apk", "add", "--no-cache", "git"}).
		WithDirectory("/src", dag.Directory()).
		WithWorkdir("/src").
		WithExec([]string{"git", "clone", gitRepository, "."})
}

func (m *Smartchangelog) LatestTag(ctx context.Context) (string, error) {
	// git describe --tags --abbrev=0
	tag, err := baseCtr().
		WithExec([]string{"git", "describe", "--tags", "--match=v[[:digit:]]*", "--abbrev=0"}).
		Stdout(ctx)

	if err != nil {
		return "", err
	}

	tag = strings.TrimRight(tag, "\n")
	return tag, nil
}

// CommitsSinceTag returns a list of commit hashes since the given tag.
func (m *Smartchangelog) CommitsSinceTag(
	ctx context.Context,
	// +optional
	tag string,
) ([]string, error) {
	// git log TAG..HEAD --oneline
	if tag == "" {
		var err error
		tag, err = m.LatestTag(ctx)
		if err != nil {
			return nil, err
		}
	}

	out, err := baseCtr().
		WithExec([]string{"git", "log", fmt.Sprintf("%s..HEAD", tag), "--oneline"}).
		Terminal().
		Stdout(ctx)
	if err != nil {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}

func (m *Smartchangelog) ReadFile(ctx context.Context, path string) (string, error) {
	return baseCtr().File(path).Contents(ctx)
}

func (m *Smartchangelog) Changelog(ctx context.Context) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY is required")
	}
	client := openai.NewClient(apiKey)

	commits, err := m.CommitsSinceTag(ctx, "v0.12.0")
	if err != nil {
		return "", err
	}

	oldChangelog, err := m.ReadFile(ctx, ".changes/v0.12.1.md")
	if err != nil {
		return "", err
	}

	userMessage := fmt.Sprintf("```%s```\n```%s```\n", oldChangelog, strings.Join(commits, "\n"))

	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleSystem,
			Content: `You will improve the changelog message of the release of Dagger, a programmable CI/CD engine.

You will be provided with the initial release message that needs to be improve, delimited by triple quotes. The release message is formatted markdown

You will also be provided with an extra list, also delimited with triple quotes. This list is composed of git commit IDs and messages, optionally ending with a Pull Request number between parenthesis.

Return an improved release message, using the commit messages that are not part of the initial release process. Ensure commit messages are not repeated.
`,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: userMessage,
		},
	}

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       "gpt-4o-mini",
			Messages:    messages,
			MaxTokens:   1000,
			Temperature: 0.9,
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return "", err
	}

	newChangelog := resp.Choices[0].Message.Content

	return newChangelog, nil
}

// Copyright 2022 SLSA Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pkg

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"

	intoto "github.com/in-toto/in-toto-golang/in_toto"
	slsa02 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	"github.com/slsa-framework/slsa-github-generator/github"
	"github.com/slsa-framework/slsa-github-generator/internal/utils"
	"github.com/slsa-framework/slsa-github-generator/signing/sigstore"
	"github.com/slsa-framework/slsa-github-generator/slsa"
)

const (
	parametersVersion  int = 1
	buildConfigVersion int = 1
	buildType              = "https://github.com/slsa-framework/slsa-github-generator/go@v1"
	requestTokenEnvKey     = "ACTIONS_ID_TOKEN_REQUEST_TOKEN"
	requestURLEnvKey       = "ACTIONS_ID_TOKEN_REQUEST_URL"
	audience               = "slsa-framework/slsa-github-generator"
)

type (
	step struct {
		Command    []string `json:"command"`
		Env        []string `json:"env"`
		WorkingDir string   `json:"workingDir"`
	}
	buildConfig struct {
		Version int    `json:"version"`
		Steps   []step `json:"steps"`
	}
)

type goProvenanceBuild struct {
	*slsa.GithubActionsBuild
	buildConfig buildConfig
}

// URI implements BuildType.URI.
func (b *goProvenanceBuild) URI() string {
	return buildType
}

// BuildConfig implements BuildType.BuildConfig.
func (b *goProvenanceBuild) BuildConfig(context.Context) (interface{}, error) {
	return b.buildConfig, nil
}

// GenerateProvenance translates github context into a SLSA provenance
// attestation.
// Spec: https://slsa.dev/provenance/v0.2
func GenerateProvenance(name, digest, command, envs, workingDir string) ([]byte, error) {
	gh, err := github.GetWorkflowContext()
	if err != nil {
		return nil, err
	}

	if _, err := hex.DecodeString(digest); err != nil || len(digest) != 64 {
		return nil, fmt.Errorf("sha256 digest is not valid: %s", digest)
	}

	com, err := UnmarshallList(command)
	if err != nil {
		return nil, err
	}

	env, err := UnmarshallList(envs)
	if err != nil {
		return nil, err
	}

	b := goProvenanceBuild{
		GithubActionsBuild: slsa.NewGithubActionsBuild([]intoto.Subject{
			{
				Name: name,
				Digest: slsa02.DigestSet{
					"sha256": digest,
				},
			},
		}, gh),
		buildConfig: buildConfig{
			Version: buildConfigVersion,
			Steps: []step{
				// Vendoring step.
				{
					// Note: vendoring and compilation are
					// performed in the same VM, so the compiler is
					// the same.
					Command:    []string{com[0], "mod", "vendor"},
					WorkingDir: workingDir,
					// Note: No user-defined env set for this step.
				},
				// Compilation step.
				{
					Command:    com,
					Env:        env,
					WorkingDir: workingDir,
				},
			},
		},
	}

	// Pre-submit tests don't have access to write OIDC token.
	if utils.IsPresubmitTests() {
		b.GithubActionsBuild.WithClients(&slsa.NilClientProvider{})
	}

	ctx := context.Background()
	g := slsa.NewHostedActionsGenerator(&b)
	// Pre-submit tests don't have access to write OIDC token.
	if utils.IsPresubmitTests() {
		g.WithClients(&slsa.NilClientProvider{})
	}
	p, err := g.Generate(ctx)
	if err != nil {
		return nil, err
	}

	// Set the architecture based on the runner. Architecture should be the
	// same for the provenance step where this is run and the build step if the
	// reusable workflow is used.
	//
	// NOTE: map is a reference so modifying invEnv modifies
	// p.Predicate.Invocation.Environment.
	invEnv := p.Predicate.Invocation.Environment.(map[string]interface{})
	invEnv["arch"] = os.Getenv("RUNNER_ARCH")
	invEnv["os"] = os.Getenv("ImageOS")

	// Add details about the runner's OS to the materials
	runnerMaterials := slsa02.ProvenanceMaterial{
		// TODO: capture the digest here too
		URI: fmt.Sprintf("https://github.com/actions/virtual-environments/releases/tag/%s/%s", os.Getenv("ImageOS"), os.Getenv("ImageVersion")),
	}
	p.Predicate.Materials = append(p.Predicate.Materials, runnerMaterials)

	if utils.IsPresubmitTests() {
		fmt.Println("Pre-submit tests detected. Skipping signing.")
		return marshallToBytes(*p)
	}

	// Sign the provenance.
	s := sigstore.NewDefaultFulcio()
	att, err := s.Sign(ctx, &intoto.Statement{
		StatementHeader: p.StatementHeader,
		Predicate:       p.Predicate,
	})
	if err != nil {
		return nil, err
	}

	// Upload the signed attestation to rekor.
	r := sigstore.NewDefaultRekor()
	if _, err := r.Upload(ctx, att); err != nil {
		return nil, err
	}

	return att.Bytes(), nil
}

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type PRCIState string

const (
	PRCINone       PRCIState = "none"
	PRCIInProgress PRCIState = "in_progress"
	PRCIFail       PRCIState = "fail"
	PRCISuccess    PRCIState = "success"
)

type PRData struct {
	Number             int
	URL                string
	Branch             string
	ReviewDecision     string
	Approved           bool
	UnresolvedComments int
	CIState            PRCIState
	CICompleted        int
	CITotal            int
}

type GHManager struct {
	mu    sync.Mutex
	cache map[string]ghRepoCache
	ttl   time.Duration
}

type ghRepoCache struct {
	fetchedAt time.Time
	prs       map[string]PRData
}

type ghPR struct {
	Number            int       `json:"number"`
	URL               string    `json:"url"`
	HeadRefName       string    `json:"headRefName"`
	ReviewDecision    string    `json:"reviewDecision"`
	StatusCheckRollup []ghCheck `json:"statusCheckRollup"`
}

type ghCheck struct {
	Conclusion string `json:"conclusion"`
	Status     string `json:"status"`
}

type ghReviewThreadsResp struct {
	Data struct {
		Repository struct {
			PullRequest struct {
				ReviewThreads struct {
					Nodes []struct {
						IsResolved bool `json:"isResolved"`
					} `json:"nodes"`
				} `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

func NewGHManager() *GHManager {
	return &GHManager{
		cache: make(map[string]ghRepoCache),
		ttl:   20 * time.Second,
	}
}

func (m *GHManager) PRDataByBranch(repoRoot string, branches []string) map[string]PRData {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" || len(branches) == 0 {
		return map[string]PRData{}
	}
	m.mu.Lock()
	cached, ok := m.cache[repoRoot]
	fresh := ok && time.Since(cached.fetchedAt) < m.ttl
	m.mu.Unlock()

	if !fresh {
		prs, err := m.fetchRepoPRData(repoRoot, branches)
		if err == nil {
			m.mu.Lock()
			m.cache[repoRoot] = ghRepoCache{fetchedAt: time.Now(), prs: prs}
			cached = m.cache[repoRoot]
			m.mu.Unlock()
		}
	}

	out := make(map[string]PRData, len(branches))
	for _, b := range branches {
		if d, ok := cached.prs[b]; ok {
			out[b] = d
		}
	}
	return out
}

func (m *GHManager) fetchRepoPRData(repoRoot string, branches []string) (map[string]PRData, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, err
	}
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(ghPath, "pr", "list", "--state", "open", "--json", "number,url,headRefName,reviewDecision,statusCheckRollup", "--limit", "200")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var prs []ghPR
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, err
	}
	owner, name, err := resolveGitHubRepo(repoRoot)
	if err != nil {
		owner, name = "", ""
	}
	wanted := make(map[string]bool, len(branches))
	for _, b := range branches {
		wanted[strings.TrimSpace(b)] = true
	}
	result := make(map[string]PRData, len(prs))
	for _, pr := range prs {
		branch := strings.TrimSpace(pr.HeadRefName)
		if branch == "" || !wanted[branch] {
			continue
		}
		ciState, ciDone, ciTotal := summarizeCI(pr.StatusCheckRollup)
		data := PRData{
			Number:         pr.Number,
			URL:            strings.TrimSpace(pr.URL),
			Branch:         branch,
			ReviewDecision: strings.TrimSpace(pr.ReviewDecision),
			Approved:       strings.EqualFold(strings.TrimSpace(pr.ReviewDecision), "approved"),
			CIState:        ciState,
			CICompleted:    ciDone,
			CITotal:        ciTotal,
		}
		if owner != "" && name != "" && pr.Number > 0 {
			if unresolved, uerr := unresolvedCommentsForPR(ghPath, repoRoot, owner, name, pr.Number); uerr == nil {
				data.UnresolvedComments = unresolved
			}
		}
		result[branch] = data
	}
	return result, nil
}

func summarizeCI(checks []ghCheck) (PRCIState, int, int) {
	if len(checks) == 0 {
		return PRCINone, 0, 0
	}
	total := 0
	completed := 0
	inProgress := false
	failed := false
	for _, c := range checks {
		status := strings.ToUpper(strings.TrimSpace(c.Status))
		conclusion := strings.ToUpper(strings.TrimSpace(c.Conclusion))
		if status == "" && conclusion == "" {
			continue
		}
		total++
		if conclusion != "" {
			completed++
			switch conclusion {
			case "SUCCESS", "SKIPPED", "NEUTRAL":
			default:
				failed = true
			}
		}
		if status != "" && status != "COMPLETED" {
			inProgress = true
		}
		if conclusion == "" {
			inProgress = true
		}
	}
	if total == 0 {
		return PRCINone, 0, 0
	}
	if failed {
		return PRCIFail, completed, total
	}
	if inProgress || completed < total {
		return PRCIInProgress, completed, total
	}
	return PRCISuccess, completed, total
}

func unresolvedCommentsForPR(ghPath string, repoRoot string, owner string, name string, number int) (int, error) {
	if owner == "" || name == "" || number <= 0 {
		return 0, errors.New("repo/number required")
	}
	query := `query($owner:String!,$name:String!,$number:Int!){repository(owner:$owner,name:$name){pullRequest(number:$number){reviewThreads(first:100){nodes{isResolved}}}}}`
	cmd := exec.Command(ghPath, "api", "graphql", "-f", "query="+query, "-F", "owner="+owner, "-F", "name="+name, "-F", fmt.Sprintf("number=%d", number))
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	var resp ghReviewThreadsResp
	if err := json.Unmarshal(out, &resp); err != nil {
		return 0, err
	}
	count := 0
	for _, t := range resp.Data.Repository.PullRequest.ReviewThreads.Nodes {
		if !t.IsResolved {
			count++
		}
	}
	return count, nil
}

func resolveGitHubRepo(repoRoot string) (string, string, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return "", "", err
	}
	remote, err := gitOutputInDir(repoRoot, gitPath, "remote", "get-url", "origin")
	if err != nil {
		return "", "", err
	}
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "", "", errors.New("origin remote missing")
	}
	if strings.HasPrefix(remote, "git@github.com:") {
		path := strings.TrimPrefix(remote, "git@github.com:")
		return splitOwnerRepo(path)
	}
	if strings.HasPrefix(remote, "https://github.com/") {
		path := strings.TrimPrefix(remote, "https://github.com/")
		return splitOwnerRepo(path)
	}
	if strings.HasPrefix(remote, "http://github.com/") {
		path := strings.TrimPrefix(remote, "http://github.com/")
		return splitOwnerRepo(path)
	}
	return "", "", errors.New("non-github origin")
}

func splitOwnerRepo(path string) (string, string, error) {
	path = strings.TrimSpace(path)
	path = strings.TrimSuffix(path, ".git")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", "", errors.New("invalid github repo path")
	}
	owner := parts[0]
	repo := parts[1]
	if owner == "" || repo == "" {
		return "", "", errors.New("invalid github repo path")
	}
	return owner, filepath.Base(repo), nil
}

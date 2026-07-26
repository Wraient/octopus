package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"net/url"
)



func GetVadapav(id string) (*Directory, error) {
	url := "https://vadapav.mov/api/d/" + id

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch directory: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	var response struct {
		Items []struct {
			Id       string `json:"id"`
			Name     string `json:"name"`
			ParentId string `json:"parentId"`
			Type     string `json:"type"`
			Size     int64  `json:"size,omitempty"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert items to Files struct slice
	var files []Files
	for _, item := range response.Items {
		files = append(files, Files{
			Id:     item.Id,
			Name:   item.Name,
			Dir:    item.Type == "folder",
			Parent: item.ParentId,
			Size:   item.Size,
		})
	}

	return &Directory{
		Path:   id,
		Name:   "",
		Parent: "",
		Files:  files,
		Id:     id,
	}, nil
}

// Helper function to extract season and episode numbers
func parseEpisodeInfo(filename string) (season, episode int) {
	// Try different patterns: S01E01, S1E1, Season 01 Episode 01
	patterns := []string{
		`[Ss](\d{1,2})[Ee](\d{1,2})`,
		`[Ss]eason\s*(\d{1,2}).*?[Ee]pisode\s*(\d{1,2})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(filename)
		if len(matches) == 3 {
			season, _ = strconv.Atoi(matches[1])
			episode, _ = strconv.Atoi(matches[2])
			return
		}
	}
	return 0, 0
}

func GetShow(id string) (*Show, error) {
	rootDir, err := GetVadapav(id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch root directory: %w", err)
	}

	show := &Show{
		Id:           id,
		Name:         rootDir.Name,
		EpisodesList: make([]EpisodeEntry, 0),
	}

	// Collect all episodes
	for _, file := range rootDir.Files {
		if !strings.HasPrefix(strings.ToLower(file.Name), "season") && 
		   !strings.HasPrefix(strings.ToLower(file.Name), "s0") && 
		   !strings.HasPrefix(strings.ToLower(file.Name), "s1") {
			continue
		}

		seasonDir, err := GetVadapav(file.Id)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch season directory: %w", err)
		}

		for _, episode := range seasonDir.Files {
			if !strings.HasSuffix(strings.ToLower(episode.Name), ".mkv") &&
			   !strings.HasSuffix(strings.ToLower(episode.Name), ".mp4") {
				continue
			}
			
			season, epNum := parseEpisodeInfo(episode.Name)
			if season > 0 && epNum > 0 {
				show.EpisodesList = append(show.EpisodesList, EpisodeEntry{
					Name:    episode.Name,
					ID:      episode.Id,
					Parent:  episode.Parent,
					Season:  season,
					Episode: epNum,
				})
			}
		}
	}

	// Sort episodes
	sort.Slice(show.EpisodesList, func(i, j int) bool {
		if show.EpisodesList[i].Season != show.EpisodesList[j].Season {
			return show.EpisodesList[i].Season < show.EpisodesList[j].Season
		}
		return show.EpisodesList[i].Episode < show.EpisodesList[j].Episode
	})

	return show, nil
}

func SearchShow(query string) ([]Directory, error) {
	// URL encode the query
	escapedQuery := url.QueryEscape(query)
	url := "https://vadapav.mov/api/s/" + escapedQuery

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	var response struct {
		Items []struct {
			Id       string `json:"id"`
			Name     string `json:"name"`
			ParentId string `json:"parentId"`
			Type     string `json:"type"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert response to Directory slice
	directories := make([]Directory, len(response.Items))
	for i, item := range response.Items {
		directories[i] = Directory{
			Path:   item.Id,
			Name:   item.Name,
			Parent: item.ParentId,
			Id:     item.Id,
		}
	}

	return directories, nil
}

func GetNextEpisode(currentShow *Show, currentEpisodeID string) *EpisodeEntry {
	// Find current episode index
	currentIndex := -1
	for i, episode := range currentShow.EpisodesList {
		if episode.ID == currentEpisodeID {
			currentIndex = i
			break
		}
	}

	// If current episode found and not the last episode
	if currentIndex != -1 && currentIndex < len(currentShow.EpisodesList)-1 {
		return &currentShow.EpisodesList[currentIndex+1]
	}

	return nil
}

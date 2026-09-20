package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const defaultCLIProfile = "default"

type cliProfileStore struct {
	Version       int                       `json:"version"`
	ActiveProfile string                    `json:"active_profile"`
	Profiles      map[string]cliCredentials `json:"profiles"`
}

type cliProfileSummary struct {
	Name      string `json:"name"`
	Active    bool   `json:"active"`
	URL       string `json:"url"`
	HouseID   string `json:"house_id,omitempty"`
	SignedIn  bool   `json:"signed_in"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

func readProfileStore() (*cliProfileStore, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return &cliProfileStore{Version: 1, ActiveProfile: defaultCLIProfile, Profiles: map[string]cliCredentials{}}, nil
	}
	if err != nil {
		return nil, err
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("credential file %s has unsafe permissions; use chmod 600", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var store cliProfileStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("read CLI profiles: %w", err)
	}
	if store.Profiles == nil {
		var legacy cliCredentials
		if err := json.Unmarshal(data, &legacy); err != nil {
			return nil, fmt.Errorf("read saved CLI session: %w", err)
		}
		legacy.Profile = defaultCLIProfile
		store = cliProfileStore{
			Version: 1, ActiveProfile: defaultCLIProfile,
			Profiles: map[string]cliCredentials{defaultCLIProfile: legacy},
		}
	}
	if store.ActiveProfile == "" {
		store.ActiveProfile = defaultCLIProfile
	}
	if store.Profiles == nil {
		store.Profiles = map[string]cliCredentials{}
	}
	for name, profile := range store.Profiles {
		profile.Profile = name
		store.Profiles[name] = profile
	}
	return &store, nil
}

func writeProfileStore(store *cliProfileStore) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create credential directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".session-*")
	if err != nil {
		return fmt.Errorf("create credential file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath) //nolint:errcheck
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(store); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write CLI profiles: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace credential file: %w", err)
	}
	return nil
}

func selectedProfileName(requested string) string {
	if name := strings.TrimSpace(requested); name != "" {
		return name
	}
	if name := strings.TrimSpace(os.Getenv("LONGHOUSE_PROFILE")); name != "" {
		return name
	}
	store, err := readProfileStore()
	if err == nil && store.ActiveProfile != "" {
		return store.ActiveProfile
	}
	return defaultCLIProfile
}

func readProfile(requested string) (*cliCredentials, error) {
	store, err := readProfileStore()
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(requested)
	if name == "" {
		name = strings.TrimSpace(os.Getenv("LONGHOUSE_PROFILE"))
	}
	if name == "" {
		name = store.ActiveProfile
	}
	profile, ok := store.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile %q does not exist; run 'longhouse profile add %s --url URL'", name, name)
	}
	profile.Profile = name
	return &profile, nil
}

func readCredentialsForProfile(requested string) (*cliCredentials, error) {
	profile, err := readProfile(requested)
	if err != nil {
		if strings.TrimSpace(requested) == "" {
			return nil, errors.New("no saved CLI session; run 'longhouse auth login --url URL'")
		}
		return nil, err
	}
	if profile.URL == "" || profile.RefreshToken == "" || profile.SessionID == "" {
		return nil, fmt.Errorf("profile %q is not signed in; run 'longhouse auth login --profile %s'", profile.Profile, profile.Profile)
	}
	return profile, nil
}

func saveProfile(credentials *cliCredentials) error {
	store, err := readProfileStore()
	if err != nil {
		return err
	}
	name := strings.TrimSpace(credentials.Profile)
	if name == "" {
		name = store.ActiveProfile
	}
	if name == "" {
		name = defaultCLIProfile
	}
	existing := store.Profiles[name]
	if credentials.HouseID == "" {
		credentials.HouseID = existing.HouseID
	}
	credentials.Profile = name
	store.Profiles[name] = *credentials
	if store.ActiveProfile == "" || len(store.Profiles) == 1 {
		store.ActiveProfile = name
	}
	store.Version = 1
	return writeProfileStore(store)
}

func removeCredentialsForProfile(requested string) error {
	store, err := readProfileStore()
	if err != nil {
		return err
	}
	name := strings.TrimSpace(requested)
	if name == "" {
		name = store.ActiveProfile
	}
	profile, ok := store.Profiles[name]
	if !ok {
		return nil
	}
	profile.Token = ""
	profile.Domain = ""
	profile.UserID = ""
	profile.DisplayName = ""
	profile.ExpiresAt = ""
	profile.RefreshToken = ""
	profile.RefreshExpiresAt = ""
	profile.SessionID = ""
	store.Profiles[name] = profile
	return writeProfileStore(store)
}

func runAuthCommand(parsed parsedCLIArgs) error {
	if len(parsed.Words) < 2 {
		return errors.New("auth requires one of: login, status, refresh, logout")
	}
	switch parsed.Words[1] {
	case "login":
		return LoginCLI(parsed.Options)
	case "status":
		return StatusCLI(parsed.Options)
	case "refresh":
		return RefreshCLI(parsed.Options)
	case "logout":
		return LogoutCLI(parsed.Options)
	default:
		return fmt.Errorf("unknown auth command %q", parsed.Words[1])
	}
}

func runProfileCommand(parsed parsedCLIArgs) error {
	if len(parsed.Words) < 2 {
		return errors.New("profile requires one of: list, view, add, use, remove, house use")
	}
	store, err := readProfileStore()
	if err != nil {
		return err
	}
	command := parsed.Words[1]
	switch command {
	case "list":
		names := make([]string, 0, len(store.Profiles))
		for name := range store.Profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		result := make([]cliProfileSummary, 0, len(names))
		for _, name := range names {
			profile := store.Profiles[name]
			result = append(result, profileSummary(name, profile, store.ActiveProfile == name))
		}
		return renderCLIResult(os.Stdout, result, parsed.Options, false)
	case "view":
		name := store.ActiveProfile
		if len(parsed.Words) > 2 {
			name = parsed.Words[2]
		}
		profile, ok := store.Profiles[name]
		if !ok {
			return fmt.Errorf("profile %q does not exist", name)
		}
		return renderCLIResult(os.Stdout, profileSummary(name, profile, store.ActiveProfile == name), parsed.Options, false)
	case "add":
		if len(parsed.Words) < 3 {
			return errors.New("profile add requires a name")
		}
		name := strings.TrimSpace(parsed.Words[2])
		if strings.ContainsAny(name, " /\\") || name == "" {
			return errors.New("profile name must not contain spaces or slashes")
		}
		if _, exists := store.Profiles[name]; exists {
			return fmt.Errorf("profile %q already exists", name)
		}
		urlValue, err := normalizeLonghouseURL(parsed.Options["url"])
		if err != nil {
			return err
		}
		store.Profiles[name] = cliCredentials{Profile: name, URL: urlValue, HouseID: parsed.Options["house"]}
		if len(store.Profiles) == 1 {
			store.ActiveProfile = name
		}
		if err := writeProfileStore(store); err != nil {
			return err
		}
		return renderCLIResult(os.Stdout, profileSummary(name, store.Profiles[name], store.ActiveProfile == name), parsed.Options, true)
	case "use":
		if len(parsed.Words) < 3 {
			return errors.New("profile use requires a name")
		}
		name := parsed.Words[2]
		if _, ok := store.Profiles[name]; !ok {
			return fmt.Errorf("profile %q does not exist", name)
		}
		store.ActiveProfile = name
		if err := writeProfileStore(store); err != nil {
			return err
		}
		return renderCLIResult(os.Stdout, profileSummary(name, store.Profiles[name], true), parsed.Options, true)
	case "remove":
		if len(parsed.Words) < 3 {
			return errors.New("profile remove requires a name")
		}
		name := parsed.Words[2]
		confirmationRuntime := &cliRuntime{input: os.Stdin, output: os.Stdout, diagnostics: os.Stderr}
		if err := confirmCLIMutation(&cliOperation{Path: []string{"profile", "remove"}, Destructive: true}, parsed.Options, confirmationRuntime); err != nil {
			return err
		}
		if _, ok := store.Profiles[name]; !ok {
			return fmt.Errorf("profile %q does not exist", name)
		}
		delete(store.Profiles, name)
		if store.ActiveProfile == name {
			store.ActiveProfile = defaultCLIProfile
			for candidate := range store.Profiles {
				store.ActiveProfile = candidate
				break
			}
		}
		if err := writeProfileStore(store); err != nil {
			return err
		}
		return renderCLIResult(os.Stdout, map[string]string{"removed": name}, parsed.Options, true)
	case "house":
		if len(parsed.Words) < 4 || parsed.Words[2] != "use" {
			return errors.New("usage: longhouse profile house use <house>")
		}
		name := selectedProfileName(parsed.Options["profile"])
		profile, ok := store.Profiles[name]
		if !ok {
			return fmt.Errorf("profile %q does not exist", name)
		}
		houseRef := parsed.Words[3]
		normalizedHouseRef, normalizeErr := normalizeTypedCLIReference("HouseID", houseRef)
		if normalizeErr != nil {
			return normalizeErr
		}
		houseRef = normalizedHouseRef
		if !looksLikeStableID(houseRef) {
			if profile.RefreshToken == "" || profile.SessionID == "" {
				return errors.New("a house name requires a signed-in profile; use a stable house identifier or sign in")
			}
			runtime, runtimeErr := newCLIRuntime(map[string]string{"profile": name})
			if runtimeErr != nil {
				return runtimeErr
			}
			profile = *runtime.credentials
			resolved, resolveErr := (&cliReferenceResolver{runtime: runtime, typeOverrides: map[string]string{}}).resolveReference("HouseID", houseRef)
			if resolveErr != nil {
				return resolveErr
			}
			houseRef = resolved
		}
		profile.HouseID = houseRef
		store.Profiles[name] = profile
		if err := writeProfileStore(store); err != nil {
			return err
		}
		return renderCLIResult(os.Stdout, profileSummary(name, profile, store.ActiveProfile == name), parsed.Options, true)
	default:
		return fmt.Errorf("unknown profile command %q", command)
	}
}

func profileSummary(name string, profile cliCredentials, active bool) cliProfileSummary {
	return cliProfileSummary{
		Name: name, Active: active, URL: profile.URL, HouseID: profile.HouseID,
		SignedIn: profile.RefreshToken != "", ExpiresAt: profile.ExpiresAt,
	}
}

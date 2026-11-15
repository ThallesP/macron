package steamhook

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ThallesP/macron/internal/failure"
)

const (
	SwitchLibraryName = "macron-steamplay.dylib"

	insertVariable    = "LSEnvironment.DYLD_INSERT_LIBRARIES"
	toolsPathVariable = "LSEnvironment.STEAM_EXTRA_COMPAT_TOOLS_PATHS"
	launchServices    = "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"

	appManagementHint = "If macOS blocked the change (\"Operation not permitted\"), allow the program you ran this from (Terminal, or Installer) under System Settings > Privacy & Security > App Management, then try again. Steam.app was left as it was"
)

type SwitchUndo string

const (
	SwitchUndoNothing  SwitchUndo = "nothing"
	SwitchUndoRestored SwitchUndo = "restored"
	SwitchUndoStripped SwitchUndo = "stripped"
)

var signedAppFiles = []string{"Contents/Info.plist", "Contents/MacOS/steam_osx", "Contents/_CodeSignature/CodeResources"}

var inputAccessServices = []string{"Accessibility", "PostEvent", "ListenEvent"}

type appState struct {
	App                string            `json:"app"`
	SwitchedFileSHA256 map[string]string `json:"switchedFileSha256"`
}

func (h Hook) appFile(relative string) string {
	return filepath.Join(h.SteamApp, filepath.FromSlash(relative))
}

func (h Hook) appLibrary() string {
	return h.appFile("Contents/MacOS/" + SwitchLibraryName)
}

func (h Hook) backupFile(relative string) string {
	return filepath.Join(h.BackupDir, filepath.Base(relative))
}

func (h Hook) statePath() string {
	return filepath.Join(h.BackupDir, "state.json")
}

func (h Hook) insertedLibraries() string {
	inserted, err := h.Run("plutil", "-extract", insertVariable, "raw", "-o", "-", h.appFile("Contents/Info.plist"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(inserted)
}

func (h Hook) isSwitchOn() bool {
	return fileExists(h.appLibrary()) && h.insertedLibraries() == h.appLibrary()
}

func (h Hook) checkSwitchCanGoOn() error {
	if !fileExists(h.appFile("Contents/Info.plist")) {
		return failure.Errorf(failure.CodeSteamAppNotFound, "no Steam.app at %s; pass --steam-app with the folder Steam is installed in", h.SteamApp)
	}
	if !fileExists(h.SwitchLibrary) {
		return failure.Errorf(failure.CodeSwitchMissing, "the Steam Play switch library is not at %s; reinstall Macron", h.SwitchLibrary)
	}

	inserted := h.insertedLibraries()
	if inserted != "" && inserted != h.appLibrary() {
		return failure.Errorf(failure.CodeSteamAppChanged, "%s already loads %s into Steam (another tool set DYLD_INSERT_LIBRARIES); remove that tool first. Nothing was changed", h.SteamApp, inserted)
	}
	return nil
}

func (h Hook) switchOn() error {
	if h.insertedLibraries() == "" {
		if err := h.backUpApp(); err != nil {
			return err
		}
	}

	if err := h.changeApp(); err != nil {
		h.restoreApp()
		return err
	}
	if err := h.writeAppState(); err != nil {
		h.restoreApp()
		return err
	}
	h.announceAppChange()
	return nil
}

func (h Hook) changeApp() error {
	if err := copyFile(h.SwitchLibrary, h.appLibrary()); err != nil {
		return failure.Errorf(failure.CodeSwitchFailed, "could not copy the switch library into %s: %v. %s", h.SteamApp, err, appManagementHint)
	}

	plist := h.appFile("Contents/Info.plist")
	if _, err := h.Run("plutil", "-extract", "LSEnvironment", "xml1", "-o", "-", plist); err != nil {
		if output, err := h.Run("plutil", "-insert", "LSEnvironment", "-dictionary", plist); err != nil {
			return failure.Errorf(failure.CodeSwitchFailed, "could not change %s: %s. %s", plist, strings.TrimSpace(output), appManagementHint)
		}
	}
	variables := map[string]string{insertVariable: h.appLibrary(), toolsPathVariable: filepath.Dir(h.toolDir())}
	for name, value := range variables {
		if output, err := h.Run("plutil", "-replace", name, "-string", value, plist); err != nil {
			return failure.Errorf(failure.CodeSwitchFailed, "could not change %s: %s. %s", plist, strings.TrimSpace(output), appManagementHint)
		}
	}

	for _, path := range []string{h.appLibrary(), h.appFile("Contents/MacOS/steam_osx"), h.SteamApp} {
		if err := h.signAdHoc(path); err != nil {
			return err
		}
	}
	return nil
}

// codesign refuses a bundle whose files carry Finder info or a resource fork
// ("resource fork, Finder information, or similar detritus not allowed").
func (h Hook) signAdHoc(path string) error {
	output, err := h.Run("codesign", "-f", "-s", "-", path)
	if err != nil && strings.Contains(output, "detritus") {
		h.Run("xattr", "-dr", "com.apple.FinderInfo", h.SteamApp)
		h.Run("xattr", "-dr", "com.apple.ResourceFork", h.SteamApp)
		output, err = h.Run("codesign", "-f", "-s", "-", path)
	}
	if err != nil {
		return failure.Errorf(failure.CodeSwitchFailed, "could not sign %s: %s. %s", path, strings.TrimSpace(output), appManagementHint)
	}
	return nil
}

func (h Hook) switchOff() (SwitchUndo, error) {
	state, hasState := h.readAppState()
	undo := SwitchUndoNothing

	isAsSwitched := hasState && h.hasAppBackup() && h.appMatches(state)
	if isAsSwitched {
		if err := h.restoreApp(); err != nil {
			return undo, err
		}
		undo = SwitchUndoRestored
	}
	if !isAsSwitched && h.insertedLibraries() == h.appLibrary() {
		if err := h.stripApp(); err != nil {
			return undo, err
		}
		undo = SwitchUndoStripped
	}

	if err := os.RemoveAll(h.BackupDir); err != nil {
		return undo, failure.Errorf(failure.CodeFileFailed, "could not remove %s: %v", h.BackupDir, err)
	}
	if undo != SwitchUndoNothing {
		h.announceAppChange()
	}
	return undo, nil
}

func (h Hook) stripApp() error {
	plist := h.appFile("Contents/Info.plist")
	for _, name := range []string{insertVariable, toolsPathVariable} {
		h.Run("plutil", "-remove", name, plist)
	}
	if h.insertedLibraries() != "" {
		return failure.Errorf(failure.CodeSwitchFailed, "could not take DYLD_INSERT_LIBRARIES out of %s; the switch library stays so Steam still starts", plist)
	}
	if err := os.Remove(h.appLibrary()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return failure.Errorf(failure.CodeFileFailed, "could not remove %s: %v", h.appLibrary(), err)
	}
	return h.signAdHoc(h.SteamApp)
}

func (h Hook) backUpApp() error {
	if err := os.RemoveAll(h.BackupDir); err != nil {
		return failure.Errorf(failure.CodeFileFailed, "could not clear %s: %v", h.BackupDir, err)
	}
	if err := os.MkdirAll(h.BackupDir, 0o700); err != nil {
		return failure.Errorf(failure.CodeFileFailed, "could not create %s: %v", h.BackupDir, err)
	}
	for _, relative := range signedAppFiles {
		if err := copyFile(h.appFile(relative), h.backupFile(relative)); err != nil {
			return failure.Errorf(failure.CodeFileFailed, "could not back up %s: %v", h.appFile(relative), err)
		}
	}
	return nil
}

func (h Hook) hasAppBackup() bool {
	for _, relative := range signedAppFiles {
		if !fileExists(h.backupFile(relative)) {
			return false
		}
	}
	return true
}

// Without a whole backup the app is left as it is: its Info.plist may name the
// switch library, and Steam does not start when that file is gone.
func (h Hook) restoreApp() error {
	if !h.hasAppBackup() {
		return nil
	}
	for _, relative := range signedAppFiles {
		if err := copyFile(h.backupFile(relative), h.appFile(relative)); err != nil {
			return failure.Errorf(failure.CodeFileFailed, "could not put %s back: %v", h.appFile(relative), err)
		}
	}
	if err := os.Remove(h.appLibrary()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return failure.Errorf(failure.CodeFileFailed, "could not remove %s: %v", h.appLibrary(), err)
	}
	return nil
}

func (h Hook) appMatches(state appState) bool {
	if state.App != h.SteamApp {
		return false
	}
	for relative, switched := range state.SwitchedFileSHA256 {
		current, err := sha256File(h.appFile(relative))
		if err != nil || current != switched {
			return false
		}
	}
	return true
}

func (h Hook) writeAppState() error {
	state := appState{App: h.SteamApp, SwitchedFileSHA256: map[string]string{}}
	for _, relative := range signedAppFiles {
		sum, err := sha256File(h.appFile(relative))
		if err != nil {
			return err
		}
		state.SwitchedFileSHA256[relative] = sum
	}

	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(h.BackupDir, 0o700); err != nil {
		return failure.Errorf(failure.CodeFileFailed, "could not create %s: %v", h.BackupDir, err)
	}
	return writeFileAtomic(h.statePath(), string(encoded)+"\n")
}

func (h Hook) readAppState() (appState, bool) {
	data, err := os.ReadFile(h.statePath())
	if err != nil {
		return appState{}, false
	}
	var state appState
	if err := json.Unmarshal(data, &state); err != nil {
		return appState{}, false
	}
	return state, true
}

// macOS keeps Steam's input permissions against its signature, so after the
// signature changes a stale approval shows as granted and silently does not
// work. Resetting makes macOS ask again.
func (h Hook) announceAppChange() {
	h.Run(launchServices, "-f", h.SteamApp)
	for _, service := range inputAccessServices {
		h.Run("tccutil", "reset", service, "com.valvesoftware.steam")
	}
}

func copyFile(from string, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	info, err := os.Stat(from)
	if err != nil {
		return err
	}
	if err := os.WriteFile(to, data, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Chmod(to, info.Mode().Perm())
}

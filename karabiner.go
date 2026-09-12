package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed config/karabiner
var karabinerRuleFiles embed.FS

const (
	karabinerRuleMarker         = "omacy: "
	karabinerRuleDir            = "config/karabiner"
	defaultKarabinerProfileName = "Default profile"
	karabinerCLIName            = "karabiner_cli"

	karabinerCallTimeout = 5 * time.Second
)

// KarabinerConfig is the karabiner.json file.
// source: https://karabiner-elements.pqrs.org/docs/json/root-data-structure/
type KarabinerConfig struct {
	Global          *KarabinerGlobal                    `json:"global,omitempty"`
	MachineSpecific map[string]KarabinerMachineSpecific `json:"machine_specific,omitempty"`
	Profiles        []KarabinerProfile                  `json:"profiles,omitempty"`
}

func (k *KarabinerConfig) Profile(name string) *KarabinerProfile {
	for i := range k.Profiles {
		if k.Profiles[i].Name == name {
			return &k.Profiles[i]
		}
	}
	return nil
}

type KarabinerGlobal struct {
	CheckForUpdates                                      *bool                              `json:"check_for_updates,omitempty"`
	ShowInMenuBar                                        *bool                              `json:"show_in_menu_bar,omitempty"`
	ShowProfileNameInMenuBar                             *bool                              `json:"show_profile_name_in_menu_bar,omitempty"`
	ShowAdditionalMenuItems                              *bool                              `json:"show_additional_menu_items,omitempty"`
	ShowQuitConfirmationMenu                             *bool                              `json:"show_quit_confirmation_menu,omitempty"`
	EnableNotificationWindow                             *bool                              `json:"enable_notification_window,omitempty"`
	NotificationWindowPosition                           string                             `json:"notification_window_position,omitempty"`
	NotificationWindowRespectScreenVisibleFrame          *bool                              `json:"notification_window_respect_screen_visible_frame,omitempty"`
	NotificationWindowShowIcon                           *bool                              `json:"notification_window_show_icon,omitempty"`
	NotificationWindowFontSize                           *int                               `json:"notification_window_font_size,omitempty"`
	NotificationWindowColors                             *KarabinerNotificationWindowColors `json:"notification_window_colors,omitempty"`
	UnsafeUI                                             *bool                              `json:"unsafe_ui,omitempty"`
	FilterUselessEventsFromSpecificDevices               *bool                              `json:"filter_useless_events_from_specific_devices,omitempty"`
	ReorderSameTimestampInputEventsToPrioritizeModifiers *bool                              `json:"reorder_same_timestamp_input_events_to_prioritize_modifiers,omitempty"`
	EnableCGEventTapFallback                             *bool                              `json:"enable_cgeventtap_fallback,omitempty"`
	DelayMillisecondsBeforeSleepShortcut                 *int                               `json:"delay_milliseconds_before_sleep_shortcut,omitempty"`
}

// KarabinerNotificationWindowColors holds colors as "system" or "#rrggbb".
type KarabinerNotificationWindowColors struct {
	BackgroundColor string `json:"background_color,omitempty"`
	TextColor       string `json:"text_color,omitempty"`
}

// KarabinerMachineSpecific holds the settings of one machine, keyed by machine identifier.
type KarabinerMachineSpecific struct {
	EnableMultitouchExtension *bool  `json:"enable_multitouch_extension,omitempty"`
	ExternalEditorPath        string `json:"external_editor_path,omitempty"`
}

type KarabinerProfile struct {
	Name                                string                         `json:"name"`
	Selected                            bool                           `json:"selected"`
	IgnorePointingDeviceEventsByDefault *bool                          `json:"ignore_pointing_device_events_by_default,omitempty"`
	Parameters                          *KarabinerProfileParameters    `json:"parameters,omitempty"`
	VirtualHIDKeyboard                  *KarabinerVirtualHIDKeyboard   `json:"virtual_hid_keyboard,omitempty"`
	SimpleModifications                 []KarabinerSimpleModification  `json:"simple_modifications,omitempty"`
	FnFunctionKeys                      []KarabinerSimpleModification  `json:"fn_function_keys,omitempty"`
	ComplexModifications                *KarabinerComplexModifications `json:"complex_modifications,omitempty"`
	Devices                             []KarabinerDevice              `json:"devices,omitempty"`
}

type KarabinerProfileParameters struct {
	DelayMillisecondsBeforeOpenDevice *int `json:"delay_milliseconds_before_open_device,omitempty"`
}

type KarabinerVirtualHIDKeyboard struct {
	KeyboardTypeV2                  string `json:"keyboard_type_v2,omitempty"`
	CountryCode                     *int   `json:"country_code,omitempty"`
	MouseKeyXYScale                 *int   `json:"mouse_key_xy_scale,omitempty"`
	IndicateStickyModifierKeysState *bool  `json:"indicate_sticky_modifier_keys_state,omitempty"`
}

// KarabinerSimpleModification is one row of the simple modifications or function keys table. From
// and To hold key event definitions, which omacy passes through untouched.
type KarabinerSimpleModification struct {
	From json.RawMessage `json:"from,omitempty"`
	To   json.RawMessage `json:"to,omitempty"`
}

type KarabinerComplexModifications struct {
	Parameters *KarabinerComplexModificationsParameters `json:"parameters,omitempty"`
	Rules      []KarabinerComplexModificationsRule      `json:"rules,omitempty"`
}

type KarabinerComplexModificationsParameters struct {
	BasicSimultaneousThresholdMilliseconds *int `json:"basic.simultaneous_threshold_milliseconds,omitempty"`
	BasicToIfAloneTimeoutMilliseconds      *int `json:"basic.to_if_alone_timeout_milliseconds,omitempty"`
	BasicToIfHeldDownThresholdMilliseconds *int `json:"basic.to_if_held_down_threshold_milliseconds,omitempty"`
	BasicToDelayedActionDelayMilliseconds  *int `json:"basic.to_delayed_action_delay_milliseconds,omitempty"`
	MouseMotionToScrollSpeed               *int `json:"mouse_motion_to_scroll.speed,omitempty"`
}

// KarabinerComplexModificationsRule is one rule. A rule either carries EvalJS, JavaScript Karabiner
// runs to build the rule, or it spells the rule out with Manipulators. Enabled is absent until the
// rule is turned off in the settings window.
type KarabinerComplexModificationsRule struct {
	EvalJS           string            `json:"eval_js,omitempty"`
	Description      string            `json:"description,omitempty"`
	DescriptionNotes []string          `json:"description_notes,omitempty"`
	Manipulators     []json.RawMessage `json:"manipulators,omitempty"`
	Enabled          *bool             `json:"enabled,omitempty"`
}

type KarabinerDevice struct {
	Identifiers                      KarabinerDeviceIdentifiers    `json:"identifiers"`
	Ignore                           *bool                         `json:"ignore,omitempty"`
	ManipulateCapsLockLED            *bool                         `json:"manipulate_caps_lock_led,omitempty"`
	SwapGraveAccentAndNonUSBackslash *bool                         `json:"swap_grave_accent_and_non_us_backslash,omitempty"`
	IgnoreVendorEvents               *bool                         `json:"ignore_vendor_events,omitempty"`
	TreatAsBuiltInKeyboard           *bool                         `json:"treat_as_built_in_keyboard,omitempty"`
	DisableBuiltInKeyboardIfExists   *bool                         `json:"disable_built_in_keyboard_if_exists,omitempty"`
	SimpleModifications              []KarabinerSimpleModification `json:"simple_modifications,omitempty"`
	FnFunctionKeys                   []KarabinerSimpleModification `json:"fn_function_keys,omitempty"`

	PointingMotionXYMultiplier     *float64 `json:"pointing_motion_xy_multiplier,omitempty"`
	PointingMotionWheelsMultiplier *float64 `json:"pointing_motion_wheels_multiplier,omitempty"`

	MouseFlipX                  *bool `json:"mouse_flip_x,omitempty"`
	MouseFlipY                  *bool `json:"mouse_flip_y,omitempty"`
	MouseFlipVerticalWheel      *bool `json:"mouse_flip_vertical_wheel,omitempty"`
	MouseFlipHorizontalWheel    *bool `json:"mouse_flip_horizontal_wheel,omitempty"`
	MouseSwapXY                 *bool `json:"mouse_swap_xy,omitempty"`
	MouseSwapWheels             *bool `json:"mouse_swap_wheels,omitempty"`
	MouseDiscardX               *bool `json:"mouse_discard_x,omitempty"`
	MouseDiscardY               *bool `json:"mouse_discard_y,omitempty"`
	MouseDiscardVerticalWheel   *bool `json:"mouse_discard_vertical_wheel,omitempty"`
	MouseDiscardHorizontalWheel *bool `json:"mouse_discard_horizontal_wheel,omitempty"`

	GamePadSwapSticks                                             *bool    `json:"game_pad_swap_sticks,omitempty"`
	GamePadXYStickDeadzone                                        *float64 `json:"game_pad_xy_stick_deadzone,omitempty"`
	GamePadXYStickDeltaMagnitudeDetectionThreshold                *float64 `json:"game_pad_xy_stick_delta_magnitude_detection_threshold,omitempty"`
	GamePadXYStickContinuedMovementAbsoluteMagnitudeThreshold     *float64 `json:"game_pad_xy_stick_continued_movement_absolute_magnitude_threshold,omitempty"`
	GamePadXYStickContinuedMovementIntervalMilliseconds           *int     `json:"game_pad_xy_stick_continued_movement_interval_milliseconds,omitempty"`
	GamePadWheelsStickDeadzone                                    *float64 `json:"game_pad_wheels_stick_deadzone,omitempty"`
	GamePadWheelsStickDeltaMagnitudeDetectionThreshold            *float64 `json:"game_pad_wheels_stick_delta_magnitude_detection_threshold,omitempty"`
	GamePadWheelsStickContinuedMovementAbsoluteMagnitudeThreshold *float64 `json:"game_pad_wheels_stick_continued_movement_absolute_magnitude_threshold,omitempty"`
	GamePadWheelsStickContinuedMovementIntervalMilliseconds       *int     `json:"game_pad_wheels_stick_continued_movement_interval_milliseconds,omitempty"`
	GamePadStickXFormula                                          string   `json:"game_pad_stick_x_formula,omitempty"`
	GamePadStickYFormula                                          string   `json:"game_pad_stick_y_formula,omitempty"`
	GamePadStickVerticalWheelFormula                              string   `json:"game_pad_stick_vertical_wheel_formula,omitempty"`
	GamePadStickHorizontalWheelFormula                            string   `json:"game_pad_stick_horizontal_wheel_formula,omitempty"`
}

// KarabinerDeviceIdentifiers picks out a device. The fields that are set all have to match.
type KarabinerDeviceIdentifiers struct {
	VendorID         *int   `json:"vendor_id,omitempty"`
	ProductID        *int   `json:"product_id,omitempty"`
	DeviceAddress    string `json:"device_address,omitempty"`
	IsKeyboard       *bool  `json:"is_keyboard,omitempty"`
	IsPointingDevice *bool  `json:"is_pointing_device,omitempty"`
	IsGamePad        *bool  `json:"is_game_pad,omitempty"`
	IsConsumer       *bool  `json:"is_consumer,omitempty"`
	IsVirtualDevice  *bool  `json:"is_virtual_device,omitempty"`
}

// Output of --show-settings-window-guidance
type karabinerGuidance struct {
	CurrentAlert           string `json:"current_alert"`
	CurrentSetup           string `json:"current_setup"`
	CoreServiceDaemonState struct {
		DriverActivated             bool `json:"driver_activated"`
		DriverConnected             bool `json:"driver_connected"`
		BundlePermissionCheckResult struct {
			AccessibilityProcessTrusted bool `json:"accessibility_process_trusted"`
			IOHIDListenEventAllowed     bool `json:"iohid_listen_event_allowed"`
		} `json:"bundle_permission_check_result"`
	} `json:"core_service_daemon_state"`
}

type karabinerApp struct {
	path string
	// rawConfig and config stay nil until karabiner.json is read or started.
	rawConfig map[string]any
	config    *KarabinerConfig
}

func (a *karabinerApp) hasConfig() bool {
	return a.config != nil
}

func karabinerConfigPath() string {
	return filepath.Join(ConfigDir, "karabiner", "karabiner.json")
}

// openKarabiner reads karabiner.json. A machine where Karabiner has never run has no
// karabiner.json, and the app comes back holding no configuration.
func openKarabiner() (*karabinerApp, error) {
	app := &karabinerApp{path: karabinerConfigPath()}

	missing, err := pathMissing(app.path)
	if err != nil {
		return nil, err
	}
	if missing {
		return app, nil
	}

	data, err := os.ReadFile(app.path)
	if err != nil {
		return nil, fmt.Errorf("cannot read the karabiner config %s: %v", app.path, err)
	}
	if err := app.decode(data); err != nil {
		return nil, err
	}
	return app, nil
}

func decodeJSON(data []byte, into any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode(into)
}

func (a *karabinerApp) decode(data []byte) error {
	if err := decodeJSON(data, &a.rawConfig); err != nil {
		return fmt.Errorf("error reading %s: %w", a.path, err)
	}
	if a.rawConfig == nil {
		return fmt.Errorf("error reading %s: it holds no configuration", a.path)
	}
	var config KarabinerConfig
	if err := decodeJSON(data, &config); err != nil {
		return fmt.Errorf("error reading %s: %w", a.path, err)
	}
	a.config = &config
	return nil
}

// createDefaultConfig puts the one profile a machine without karabiner.json needs into the app,
// with the omacy rules already in it.
func (a *karabinerApp) createDefaultConfig(profileName string, rules []KarabinerComplexModificationsRule) error {
	data, err := json.Marshal(KarabinerConfig{
		Profiles: []KarabinerProfile{
			{
				Name:                 profileName,
				Selected:             true,
				VirtualHIDKeyboard:   &KarabinerVirtualHIDKeyboard{KeyboardTypeV2: "ansi"},
				ComplexModifications: &KarabinerComplexModifications{Rules: rules},
			},
		},
	})
	if err != nil {
		return err
	}
	return a.decode(data)
}

func rawObject(parent map[string]any, key string) (map[string]any, error) {
	value, ok := parent[key]
	if !ok || value == nil {
		return map[string]any{}, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a JSON object", key)
	}
	return object, nil
}

func rawList(parent map[string]any, key string) ([]any, error) {
	value, ok := parent[key]
	if !ok || value == nil {
		return nil, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a JSON list", key)
	}
	return items, nil
}

func (a *karabinerApp) rawProfile(name string) (map[string]any, error) {
	profiles, err := rawList(a.rawConfig, "profiles")
	if err != nil {
		return nil, err
	}
	for i, entry := range profiles {
		profile, ok := entry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("profile %d of %s is not a JSON object", i, a.path)
		}
		if profile["name"] == name {
			return profile, nil
		}
	}
	return nil, fmt.Errorf("karabiner runs profile %q, which is not in %s", name, a.path)
}

func writtenByOmacy(rule any) bool {
	fields, ok := rule.(map[string]any)
	if !ok {
		return false
	}
	description, ok := fields["description"].(string)
	return ok && strings.HasPrefix(description, karabinerRuleMarker)
}

func (a *karabinerApp) setOmacyRules(profileName string, omacyRules []KarabinerComplexModificationsRule) error {
	profile, err := a.rawProfile(profileName)
	if err != nil {
		return err
	}
	modifications, err := rawObject(profile, "complex_modifications")
	if err != nil {
		return err
	}
	rules, err := rawList(modifications, "rules")
	if err != nil {
		return err
	}

	kept := make([]any, 0, len(rules)+len(omacyRules))
	for _, rule := range rules {
		if writtenByOmacy(rule) {
			continue
		}
		kept = append(kept, rule)
	}
	for _, rule := range omacyRules {
		kept = append(kept, rule)
	}

	modifications["rules"] = kept
	profile["complex_modifications"] = modifications
	return nil
}

func (a *karabinerApp) write() error {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "    ")
	if err := encoder.Encode(a.rawConfig); err != nil {
		return fmt.Errorf("error encoding %s: %w", a.path, err)
	}
	return writeFile(a.path, out.Bytes(), 0o644)
}

func loadKarabinerRules() ([]KarabinerComplexModificationsRule, error) {
	paths, err := fs.Glob(karabinerRuleFiles, karabinerRuleDir+"/*.js")
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	rules := make([]KarabinerComplexModificationsRule, 0, len(paths))
	for _, path := range paths {
		source, err := fs.ReadFile(karabinerRuleFiles, path)
		if err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(filepath.Base(path), ".js")
		rules = append(rules, KarabinerComplexModificationsRule{
			EvalJS:      string(source),
			Description: karabinerRuleMarker + name,
		})
	}
	return rules, nil
}

func runKarabiner(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, karabinerCallTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, karabinerCLIName, args...)
	cmd.WaitDelay = karabinerCallTimeout

	out, err := cmd.Output()
	asked := karabinerCLIName + " " + strings.Join(args, " ")
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("%s timed out %s", asked, karabinerCallTimeout)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("%s failed: %w: %s", asked, err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	printed := strings.TrimSpace(string(out))
	if message, failed := karabinerErrorLine(printed); failed {
		return "", fmt.Errorf("%s failed: %s", asked, message)
	}
	return printed, nil
}

func karabinerErrorLine(printed string) (string, bool) {
	for line := range strings.SplitSeq(printed, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[error]") {
			return line, true
		}
	}
	return "", false
}

// profileName is the profile omacy writes its rules into: the one Karabiner runs, or the one
// karabiner.json marks as selected, or the name Karabiner gives a fresh profile.
func (a *karabinerApp) profileName(ctx context.Context) string {
	name, err := runKarabiner(ctx, "--show-current-profile-name")
	if err != nil {
		slog.Debug("Cannot ask Karabiner which profile it runs", "err", err)
	}
	if name != "" {
		return name
	}
	if !a.hasConfig() {
		return defaultKarabinerProfileName
	}
	for _, profile := range a.config.Profiles {
		if profile.Selected && profile.Name != "" {
			return profile.Name
		}
	}
	return defaultKarabinerProfileName
}

func (a *karabinerApp) selectProfile(ctx context.Context, name string) error {
	_, err := runKarabiner(ctx, "--select-profile", name)
	return err
}

func karabinerPermission(ctx context.Context) (permissionStatus, error) {
	out, err := runKarabiner(ctx, "--show-settings-window-guidance")
	if err != nil {
		return permissionUnknown, err
	}
	var g karabinerGuidance
	if err := json.Unmarshal([]byte(out), &g); err != nil {
		return permissionUnknown, fmt.Errorf("cannot read what %s said about its permissions: %w", karabinerCLIName, err)
	}
	state := g.CoreServiceDaemonState
	permission := state.BundlePermissionCheckResult
	if g.CurrentAlert == "none" &&
		g.CurrentSetup == "none" &&
		state.DriverActivated &&
		state.DriverConnected &&
		permission.AccessibilityProcessTrusted &&
		permission.IOHIDListenEventAllowed {
		return permissionGranted, nil
	}
	return permissionDenied, nil
}

func setupKarabiner(ctx context.Context) error {
	app, err := openKarabiner()
	if err != nil {
		return err
	}

	rules, err := loadKarabinerRules()
	if err != nil {
		return fmt.Errorf("cannot load shipped rules: %v", err)
	}

	profileName := app.profileName(ctx)
	if app.hasConfig() {
		err = app.setOmacyRules(profileName, rules)
	} else {
		err = app.createDefaultConfig(profileName, rules)
	}
	if err != nil {
		return err
	}
	if err := app.write(); err != nil {
		return err
	}

	if err := app.selectProfile(ctx, profileName); err != nil {
		slog.Warn("Karabiner was not told to run the profile omacy wrote, its shortcuts start working once that profile is picked", "profile", profileName, "err", err)
	}
	return nil
}

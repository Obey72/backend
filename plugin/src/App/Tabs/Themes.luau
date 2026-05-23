--!strict
-- themes tab  built-in themes community themes (pending owner validation) custom theme builder
-- built-in themes are hardcoded here and never saved to plugin settings
-- community themes have a "pending" flag stripped by the owner before they ship
-- custom themes are user-built via the assistant and stored via themeadd()

local UiLibrary = require("../../UiLibrary")
local Theme = require("../../UiLibrary/Theme")

-- built-in themes
-- each entry: name (required) + any theme field you want to override
-- color3 fields as {r g b} (0-255) studiostyleguide as {"enumname"}
-- font as "familyname" string or {"family" "weight" "style"}

local builtins: { { [string]: any } } = {

	-- 1 midnight slate  deep navy cyan pop
	{
		Name = "midnight slate",
		Font = { "JetBrainsMono", "Regular", "Normal" },
		TextSize = 13,
		BackgroundColor = { 10, 12, 24 },
		ForegroundColor = { 16, 20, 36 },
		BorderColor = { 36, 44, 72 },
		UnselectedColor = { 24, 28, 48 },
		SelectedColor = { 80, 200, 230 },
		TextColor = { 210, 220, 240 },
		DimmedTextColor = { 90, 100, 130 },
		ToggleBackgroundColor = { 80, 200, 230 },
		TipColor = { 80, 200, 230 },
		InputColor = { 210, 220, 240 },
		StrokeTransparency = 1,
	},

	-- 2 sakura light  soft pink warm white
	{
		Name = "sakura light",
		Font = { "Nunito", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 255, 248, 250 },
		ForegroundColor = { 255, 240, 244 },
		BorderColor = { 240, 200, 215 },
		UnselectedColor = { 255, 228, 236 },
		SelectedColor = { 220, 100, 140 },
		TextColor = { 60, 30, 45 },
		DimmedTextColor = { 170, 130, 150 },
		ToggleBackgroundColor = { 220, 100, 140 },
		TipColor = { 180, 70, 110 },
		InputColor = { 60, 30, 45 },
		StrokeTransparency = 1,
	},

	-- 3 forest terminal  dark green bright lime
	{
		Name = "forest terminal",
		Font = { "RobotoMono", "Regular", "Normal" },
		TextSize = 13,
		BackgroundColor = { 8, 14, 8 },
		ForegroundColor = { 12, 22, 12 },
		BorderColor = { 28, 52, 28 },
		UnselectedColor = { 18, 34, 18 },
		SelectedColor = { 80, 220, 60 },
		TextColor = { 160, 240, 160 },
		DimmedTextColor = { 70, 120, 70 },
		ToggleBackgroundColor = { 80, 220, 60 },
		TipColor = { 80, 220, 60 },
		InputColor = { 160, 240, 160 },
		StrokeTransparency = 1,
	},

	-- 4 cream linen  warm off-white amber accent
	{
		Name = "cream linen",
		Font = { "Merriweather", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 252, 245, 230 },
		ForegroundColor = { 245, 236, 218 },
		BorderColor = { 220, 200, 170 },
		UnselectedColor = { 238, 224, 196 },
		SelectedColor = { 200, 130, 40 },
		TextColor = { 50, 35, 15 },
		DimmedTextColor = { 160, 140, 110 },
		ToggleBackgroundColor = { 200, 130, 40 },
		TipColor = { 180, 110, 30 },
		InputColor = { 50, 35, 15 },
		StrokeTransparency = 1,
	},

	-- 5 rose gold  blush bg muted gold charcoal text
	{
		Name = "rose gold",
		Font = { "Gotham", "Medium", "Normal" },
		TextSize = 13,
		BackgroundColor = { 42, 32, 36 },
		ForegroundColor = { 54, 42, 46 },
		BorderColor = { 90, 68, 72 },
		UnselectedColor = { 68, 52, 56 },
		SelectedColor = { 220, 168, 120 },
		TextColor = { 240, 224, 216 },
		DimmedTextColor = { 150, 120, 112 },
		ToggleBackgroundColor = { 220, 168, 120 },
		TipColor = { 220, 168, 120 },
		InputColor = { 240, 224, 216 },
		StrokeTransparency = 1,
	},

	-- 6 void purple  near-black bg electric violet
	{
		Name = "void purple",
		Font = { "JetBrainsMono", "Light", "Normal" },
		TextSize = 13,
		BackgroundColor = { 10, 6, 18 },
		ForegroundColor = { 18, 12, 32 },
		BorderColor = { 48, 30, 80 },
		UnselectedColor = { 28, 18, 52 },
		SelectedColor = { 160, 80, 255 },
		TextColor = { 220, 210, 240 },
		DimmedTextColor = { 100, 85, 140 },
		ToggleBackgroundColor = { 160, 80, 255 },
		TipColor = { 160, 80, 255 },
		InputColor = { 220, 210, 240 },
		StrokeTransparency = 1,
	},

	-- 7 arctic frost  ice blue-white deep navy accent
	{
		Name = "arctic frost",
		Font = { "SourceSansPro", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 236, 244, 252 },
		ForegroundColor = { 220, 234, 248 },
		BorderColor = { 180, 210, 235 },
		UnselectedColor = { 200, 222, 244 },
		SelectedColor = { 30, 80, 180 },
		TextColor = { 20, 40, 80 },
		DimmedTextColor = { 110, 140, 180 },
		ToggleBackgroundColor = { 30, 80, 180 },
		TipColor = { 30, 80, 180 },
		InputColor = { 20, 40, 80 },
		StrokeTransparency = 1,
	},

	-- 8 ember  near-black with deep orange glow
	{
		Name = "ember",
		Font = { "Oswald", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 14, 8, 4 },
		ForegroundColor = { 24, 14, 8 },
		BorderColor = { 70, 35, 15 },
		UnselectedColor = { 38, 22, 10 },
		SelectedColor = { 255, 120, 30 },
		TextColor = { 255, 220, 180 },
		DimmedTextColor = { 160, 100, 60 },
		ToggleBackgroundColor = { 255, 120, 30 },
		TipColor = { 255, 120, 30 },
		InputColor = { 255, 220, 180 },
		StrokeTransparency = 1,
	},

	-- 9 pastel dream  lavender bg soft multi accents
	{
		Name = "pastel dream",
		Font = { "Nunito", "Light", "Normal" },
		TextSize = 14,
		BackgroundColor = { 240, 234, 252 },
		ForegroundColor = { 232, 224, 248 },
		BorderColor = { 210, 195, 240 },
		UnselectedColor = { 220, 210, 244 },
		SelectedColor = { 180, 140, 230 },
		TextColor = { 60, 40, 90 },
		DimmedTextColor = { 160, 145, 190 },
		ToggleBackgroundColor = { 180, 140, 230 },
		TipColor = { 180, 140, 230 },
		InputColor = { 60, 40, 90 },
		StrokeTransparency = 1,
	},

	-- 10 carbon  dark charcoal pure white
	{
		Name = "carbon",
		Font = { "Gotham", "Regular", "Normal" },
		TextSize = 13,
		BackgroundColor = { 20, 20, 20 },
		ForegroundColor = { 30, 30, 30 },
		BorderColor = { 60, 60, 60 },
		UnselectedColor = { 42, 42, 42 },
		SelectedColor = { 255, 255, 255 },
		TextColor = { 240, 240, 240 },
		DimmedTextColor = { 120, 120, 120 },
		ToggleBackgroundColor = { 255, 255, 255 },
		TipColor = { 255, 255, 255 },
		InputColor = { 240, 240, 240 },
		StrokeTransparency = 1,
	},

	-- 11 neon city  very dark bg hot pink accent
	{
		Name = "neon city",
		Font = { "RobotoMono", "Bold", "Normal" },
		TextSize = 13,
		BackgroundColor = { 6, 4, 16 },
		ForegroundColor = { 12, 8, 28 },
		BorderColor = { 50, 20, 80 },
		UnselectedColor = { 22, 14, 44 },
		SelectedColor = { 255, 40, 160 },
		TextColor = { 230, 210, 255 },
		DimmedTextColor = { 110, 80, 160 },
		ToggleBackgroundColor = { 255, 40, 160 },
		TipColor = { 255, 40, 160 },
		InputColor = { 230, 210, 255 },
		StrokeTransparency = 1,
	},

	-- 12 sunrise  warm yellow-orange gradient-ish dark text
	{
		Name = "sunrise",
		Font = { "Merriweather", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 255, 245, 220 },
		ForegroundColor = { 255, 232, 190 },
		BorderColor = { 240, 190, 120 },
		UnselectedColor = { 255, 218, 165 },
		SelectedColor = { 220, 80, 20 },
		TextColor = { 50, 25, 10 },
		DimmedTextColor = { 180, 130, 80 },
		ToggleBackgroundColor = { 220, 80, 20 },
		TipColor = { 200, 60, 10 },
		InputColor = { 50, 25, 10 },
		StrokeTransparency = 1,
	},

	-- 13 ocean deep  dark teal-blue aqua pop
	{
		Name = "ocean deep",
		Font = { "SourceSansPro", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 4, 20, 30 },
		ForegroundColor = { 8, 30, 46 },
		BorderColor = { 20, 65, 90 },
		UnselectedColor = { 12, 46, 66 },
		SelectedColor = { 0, 210, 200 },
		TextColor = { 190, 235, 240 },
		DimmedTextColor = { 80, 150, 170 },
		ToggleBackgroundColor = { 0, 210, 200 },
		TipColor = { 0, 210, 200 },
		InputColor = { 190, 235, 240 },
		StrokeTransparency = 1,
	},

	-- 14 chalk board  very dark green white chalk text
	{
		Name = "chalk board",
		Font = { "Inconsolata", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 32, 52, 38 },
		ForegroundColor = { 42, 66, 48 },
		BorderColor = { 80, 110, 90 },
		UnselectedColor = { 56, 82, 64 },
		SelectedColor = { 240, 240, 235 },
		TextColor = { 240, 240, 230 },
		DimmedTextColor = { 160, 175, 165 },
		ToggleBackgroundColor = { 240, 240, 235 },
		TipColor = { 220, 230, 210 },
		InputColor = { 240, 240, 230 },
		StrokeTransparency = 1,
	},

	-- 15 synthwave  deep magenta dark hot purple + teal
	{
		Name = "synthwave",
		Font = { "Rajdhani", "SemiBold", "Normal" },
		TextSize = 14,
		BackgroundColor = { 16, 6, 28 },
		ForegroundColor = { 26, 10, 46 },
		BorderColor = { 80, 20, 120 },
		UnselectedColor = { 44, 14, 74 },
		SelectedColor = { 0, 230, 200 },
		TextColor = { 240, 200, 255 },
		DimmedTextColor = { 130, 80, 180 },
		ToggleBackgroundColor = { 200, 0, 255 },
		TipColor = { 0, 230, 200 },
		InputColor = { 240, 200, 255 },
		StrokeTransparency = 1,
	},

	-- 16 warm paper  old paper off-white ink brown
	{
		Name = "warm paper",
		Font = { "Merriweather", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 245, 236, 210 },
		ForegroundColor = { 238, 226, 194 },
		BorderColor = { 200, 178, 140 },
		UnselectedColor = { 228, 212, 178 },
		SelectedColor = { 120, 60, 20 },
		TextColor = { 40, 20, 8 },
		DimmedTextColor = { 160, 125, 90 },
		ToggleBackgroundColor = { 120, 60, 20 },
		TipColor = { 100, 50, 15 },
		InputColor = { 40, 20, 8 },
		StrokeTransparency = 1,
	},

	-- 17 ice cream  mint + cream soft duo
	{
		Name = "ice cream",
		Font = { "Nunito", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 225, 250, 245 },
		ForegroundColor = { 210, 245, 238 },
		BorderColor = { 170, 225, 210 },
		UnselectedColor = { 190, 238, 226 },
		SelectedColor = { 50, 180, 160 },
		TextColor = { 20, 70, 60 },
		DimmedTextColor = { 110, 175, 165 },
		ToggleBackgroundColor = { 50, 180, 160 },
		TipColor = { 50, 180, 160 },
		InputColor = { 20, 70, 60 },
		StrokeTransparency = 1,
	},

	-- 18 blood moon  deep crimson bg pale red text
	{
		Name = "blood moon",
		Font = { "Oswald", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 14, 4, 4 },
		ForegroundColor = { 24, 8, 8 },
		BorderColor = { 80, 20, 20 },
		UnselectedColor = { 40, 12, 12 },
		SelectedColor = { 220, 40, 40 },
		TextColor = { 255, 200, 200 },
		DimmedTextColor = { 160, 80, 80 },
		ToggleBackgroundColor = { 220, 40, 40 },
		TipColor = { 220, 40, 40 },
		InputColor = { 255, 200, 200 },
		StrokeTransparency = 1,
	},

	-- 19 stormy grey  cool mid-grey electric yellow
	{
		Name = "stormy grey",
		Font = { "SourceSansPro", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 38, 42, 50 },
		ForegroundColor = { 50, 55, 65 },
		BorderColor = { 80, 88, 104 },
		UnselectedColor = { 64, 70, 82 },
		SelectedColor = { 250, 220, 20 },
		TextColor = { 230, 230, 235 },
		DimmedTextColor = { 130, 135, 150 },
		ToggleBackgroundColor = { 250, 220, 20 },
		TipColor = { 250, 220, 20 },
		InputColor = { 230, 230, 235 },
		StrokeTransparency = 1,
	},

	-- 20 space grey (apple-ish)  medium grey white
	{
		Name = "space grey",
		Font = { "Gotham", "Regular", "Normal" },
		TextSize = 13,
		BackgroundColor = { 48, 50, 55 },
		ForegroundColor = { 60, 62, 68 },
		BorderColor = { 90, 92, 100 },
		UnselectedColor = { 72, 74, 82 },
		SelectedColor = { 240, 240, 245 },
		TextColor = { 240, 240, 245 },
		DimmedTextColor = { 140, 140, 150 },
		ToggleBackgroundColor = { 200, 200, 210 },
		TipColor = { 200, 200, 210 },
		InputColor = { 240, 240, 245 },
		StrokeTransparency = 1,
	},

	-- 21 dracula  the classic because it had to be here
	{
		Name = "dracula",
		Font = { "Inconsolata", "Regular", "Normal" },
		TextSize = 13,
		BackgroundColor = { 40, 42, 54 },
		ForegroundColor = { 50, 52, 65 },
		BorderColor = { 68, 71, 90 },
		UnselectedColor = { 60, 62, 78 },
		SelectedColor = { 189, 147, 249 },
		TextColor = { 248, 248, 242 },
		DimmedTextColor = { 140, 130, 160 },
		ToggleBackgroundColor = { 80, 250, 123 },
		TipColor = { 255, 184, 108 },
		InputColor = { 248, 248, 242 },
		StrokeTransparency = 1,
	},

	-- 22 soft mono  creamy light with monospace charm
	{
		Name = "soft mono",
		Font = { "RobotoMono", "Regular", "Normal" },
		TextSize = 13,
		BackgroundColor = { 248, 244, 238 },
		ForegroundColor = { 240, 234, 226 },
		BorderColor = { 210, 198, 180 },
		UnselectedColor = { 228, 218, 202 },
		SelectedColor = { 80, 100, 180 },
		TextColor = { 30, 30, 40 },
		DimmedTextColor = { 150, 140, 120 },
		ToggleBackgroundColor = { 80, 100, 180 },
		TipColor = { 80, 100, 180 },
		InputColor = { 30, 30, 40 },
		StrokeTransparency = 1,
	},

	-- 23 cobalt  deep cobalt blue white hot
	{
		Name = "cobalt",
		Font = { "Rajdhani", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 12, 20, 60 },
		ForegroundColor = { 18, 30, 88 },
		BorderColor = { 40, 65, 160 },
		UnselectedColor = { 28, 48, 120 },
		SelectedColor = { 255, 255, 255 },
		TextColor = { 200, 215, 255 },
		DimmedTextColor = { 100, 120, 200 },
		ToggleBackgroundColor = { 255, 255, 255 },
		TipColor = { 180, 200, 255 },
		InputColor = { 200, 215, 255 },
		StrokeTransparency = 1,
	},

	-- 24 honeydew  pale green-white deep forest accent
	{
		Name = "honeydew",
		Font = { "Nunito", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 240, 252, 240 },
		ForegroundColor = { 228, 246, 228 },
		BorderColor = { 190, 228, 190 },
		UnselectedColor = { 210, 240, 210 },
		SelectedColor = { 30, 130, 50 },
		TextColor = { 20, 60, 30 },
		DimmedTextColor = { 110, 170, 120 },
		ToggleBackgroundColor = { 30, 130, 50 },
		TipColor = { 30, 130, 50 },
		InputColor = { 20, 60, 30 },
		StrokeTransparency = 1,
	},

	-- 25 coffee  espresso bg milk white
	{
		Name = "coffee",
		Font = { "Merriweather", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 22, 14, 10 },
		ForegroundColor = { 34, 22, 16 },
		BorderColor = { 80, 54, 38 },
		UnselectedColor = { 50, 34, 24 },
		SelectedColor = { 240, 210, 170 },
		TextColor = { 245, 225, 195 },
		DimmedTextColor = { 160, 125, 90 },
		ToggleBackgroundColor = { 200, 150, 90 },
		TipColor = { 200, 150, 90 },
		InputColor = { 245, 225, 195 },
		StrokeTransparency = 1,
	},

	-- 26 matrix  classic terminal green-on-black
	{
		Name = "matrix",
		Font = { "Inconsolata", "Regular", "Normal" },
		TextSize = 13,
		BackgroundColor = { 0, 8, 0 },
		ForegroundColor = { 0, 14, 0 },
		BorderColor = { 0, 50, 0 },
		UnselectedColor = { 0, 24, 0 },
		SelectedColor = { 0, 255, 65 },
		TextColor = { 0, 220, 55 },
		DimmedTextColor = { 0, 110, 28 },
		ToggleBackgroundColor = { 0, 255, 65 },
		TipColor = { 0, 255, 65 },
		InputColor = { 0, 200, 50 },
		StrokeTransparency = 1,
	},

	-- 27 dusty rose  muted pinkish-brown burgundy
	{
		Name = "dusty rose",
		Font = { "Merriweather", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 240, 225, 225 },
		ForegroundColor = { 230, 210, 212 },
		BorderColor = { 200, 170, 175 },
		UnselectedColor = { 218, 196, 200 },
		SelectedColor = { 140, 40, 60 },
		TextColor = { 60, 20, 30 },
		DimmedTextColor = { 170, 130, 140 },
		ToggleBackgroundColor = { 140, 40, 60 },
		TipColor = { 120, 30, 50 },
		InputColor = { 60, 20, 30 },
		StrokeTransparency = 1,
	},

	-- 28 steel  industrial cool-grey safety orange
	{
		Name = "steel",
		Font = { "Oswald", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 30, 35, 40 },
		ForegroundColor = { 42, 48, 55 },
		BorderColor = { 80, 90, 102 },
		UnselectedColor = { 56, 64, 74 },
		SelectedColor = { 240, 110, 20 },
		TextColor = { 220, 228, 236 },
		DimmedTextColor = { 120, 130, 145 },
		ToggleBackgroundColor = { 240, 110, 20 },
		TipColor = { 240, 110, 20 },
		InputColor = { 220, 228, 236 },
		StrokeTransparency = 1,
	},

	-- 29 mojave  desert sand cactus green
	{
		Name = "mojave",
		Font = { "Merriweather", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 245, 230, 200 },
		ForegroundColor = { 238, 220, 185 },
		BorderColor = { 200, 178, 140 },
		UnselectedColor = { 225, 206, 165 },
		SelectedColor = { 60, 130, 60 },
		TextColor = { 40, 28, 14 },
		DimmedTextColor = { 160, 138, 100 },
		ToggleBackgroundColor = { 60, 130, 60 },
		TipColor = { 50, 110, 50 },
		InputColor = { 40, 28, 14 },
		StrokeTransparency = 1,
	},

	-- 30 blueberry  deep navy-blue light periwinkle
	{
		Name = "blueberry",
		Font = { "Nunito", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 16, 22, 50 },
		ForegroundColor = { 24, 34, 70 },
		BorderColor = { 56, 72, 140 },
		UnselectedColor = { 38, 52, 100 },
		SelectedColor = { 160, 180, 255 },
		TextColor = { 210, 220, 255 },
		DimmedTextColor = { 100, 120, 200 },
		ToggleBackgroundColor = { 160, 180, 255 },
		TipColor = { 160, 180, 255 },
		InputColor = { 210, 220, 255 },
		StrokeTransparency = 1,
	},

	-- 31 mocha  rich brown warm cream
	{
		Name = "mocha",
		Font = { "Merriweather", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 38, 22, 14 },
		ForegroundColor = { 56, 34, 22 },
		BorderColor = { 110, 70, 46 },
		UnselectedColor = { 74, 46, 30 },
		SelectedColor = { 245, 210, 160 },
		TextColor = { 245, 220, 180 },
		DimmedTextColor = { 170, 125, 88 },
		ToggleBackgroundColor = { 200, 150, 90 },
		TipColor = { 200, 150, 90 },
		InputColor = { 245, 220, 180 },
		StrokeTransparency = 1,
	},

	-- 32 ice cap  glacial blue-white deep teal
	{
		Name = "ice cap",
		Font = { "SourceSansPro", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 235, 248, 255 },
		ForegroundColor = { 220, 240, 252 },
		BorderColor = { 170, 215, 240 },
		UnselectedColor = { 200, 232, 250 },
		SelectedColor = { 0, 140, 180 },
		TextColor = { 15, 50, 80 },
		DimmedTextColor = { 100, 160, 200 },
		ToggleBackgroundColor = { 0, 140, 180 },
		TipColor = { 0, 140, 180 },
		InputColor = { 15, 50, 80 },
		StrokeTransparency = 1,
	},

	-- 33 foxfire  dark bright amber-orange
	{
		Name = "foxfire",
		Font = { "Rajdhani", "SemiBold", "Normal" },
		TextSize = 14,
		BackgroundColor = { 18, 10, 4 },
		ForegroundColor = { 30, 18, 8 },
		BorderColor = { 90, 50, 20 },
		UnselectedColor = { 48, 28, 12 },
		SelectedColor = { 255, 160, 20 },
		TextColor = { 255, 220, 160 },
		DimmedTextColor = { 170, 110, 50 },
		ToggleBackgroundColor = { 255, 160, 20 },
		TipColor = { 255, 140, 10 },
		InputColor = { 255, 220, 160 },
		StrokeTransparency = 1,
	},

	-- 34 cotton candy  very light pink+purple vivid fuchsia
	{
		Name = "cotton candy",
		Font = { "Nunito", "Light", "Normal" },
		TextSize = 14,
		BackgroundColor = { 255, 240, 255 },
		ForegroundColor = { 250, 228, 252 },
		BorderColor = { 230, 190, 240 },
		UnselectedColor = { 244, 210, 250 },
		SelectedColor = { 220, 40, 200 },
		TextColor = { 80, 20, 80 },
		DimmedTextColor = { 180, 130, 190 },
		ToggleBackgroundColor = { 220, 40, 200 },
		TipColor = { 200, 30, 180 },
		InputColor = { 80, 20, 80 },
		StrokeTransparency = 1,
	},

	-- 35 typewriter  off-white faded black ink
	{
		Name = "typewriter",
		Font = { "Inconsolata", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 245, 242, 234 },
		ForegroundColor = { 236, 232, 220 },
		BorderColor = { 200, 192, 170 },
		UnselectedColor = { 220, 215, 198 },
		SelectedColor = { 40, 36, 28 },
		TextColor = { 30, 26, 20 },
		DimmedTextColor = { 150, 140, 120 },
		ToggleBackgroundColor = { 40, 36, 28 },
		TipColor = { 40, 36, 28 },
		InputColor = { 30, 26, 20 },
		StrokeTransparency = 1,
	},

	-- 36 navy + gold  classic prestige combo
	{
		Name = "navy and gold",
		Font = { "Gotham", "Medium", "Normal" },
		TextSize = 13,
		BackgroundColor = { 10, 18, 44 },
		ForegroundColor = { 16, 28, 64 },
		BorderColor = { 50, 70, 150 },
		UnselectedColor = { 28, 44, 100 },
		SelectedColor = { 220, 180, 60 },
		TextColor = { 220, 228, 255 },
		DimmedTextColor = { 100, 118, 200 },
		ToggleBackgroundColor = { 220, 180, 60 },
		TipColor = { 200, 160, 40 },
		InputColor = { 220, 228, 255 },
		StrokeTransparency = 1,
	},

	-- 37 glacier  cool whites with deep ice-blue
	{
		Name = "glacier",
		Font = { "SourceSansPro", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 230, 245, 252 },
		ForegroundColor = { 215, 236, 248 },
		BorderColor = { 160, 210, 235 },
		UnselectedColor = { 195, 228, 246 },
		SelectedColor = { 0, 110, 180 },
		TextColor = { 10, 40, 70 },
		DimmedTextColor = { 100, 155, 195 },
		ToggleBackgroundColor = { 0, 110, 180 },
		TipColor = { 0, 110, 180 },
		InputColor = { 10, 40, 70 },
		StrokeTransparency = 1,
	},

	-- 38 slime green  radioactive neon green
	{
		Name = "slime green",
		Font = { "RobotoMono", "Bold", "Normal" },
		TextSize = 13,
		BackgroundColor = { 4, 12, 4 },
		ForegroundColor = { 8, 20, 8 },
		BorderColor = { 30, 80, 30 },
		UnselectedColor = { 14, 44, 14 },
		SelectedColor = { 110, 255, 60 },
		TextColor = { 160, 255, 120 },
		DimmedTextColor = { 60, 150, 40 },
		ToggleBackgroundColor = { 110, 255, 60 },
		TipColor = { 100, 240, 50 },
		InputColor = { 160, 255, 120 },
		StrokeTransparency = 1,
	},

	-- 39 plum  deep purple-maroon off white text
	{
		Name = "plum",
		Font = { "Nunito", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 30, 12, 36 },
		ForegroundColor = { 44, 18, 54 },
		BorderColor = { 100, 44, 120 },
		UnselectedColor = { 60, 26, 76 },
		SelectedColor = { 230, 180, 255 },
		TextColor = { 238, 222, 248 },
		DimmedTextColor = { 155, 120, 185 },
		ToggleBackgroundColor = { 200, 100, 240 },
		TipColor = { 200, 100, 240 },
		InputColor = { 238, 222, 248 },
		StrokeTransparency = 1,
	},

	-- 40 solar  warm yellow bg deep brown
	{
		Name = "solar",
		Font = { "Merriweather", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 253, 246, 200 },
		ForegroundColor = { 250, 238, 170 },
		BorderColor = { 220, 196, 100 },
		UnselectedColor = { 244, 225, 140 },
		SelectedColor = { 120, 60, 10 },
		TextColor = { 60, 35, 8 },
		DimmedTextColor = { 180, 150, 80 },
		ToggleBackgroundColor = { 180, 100, 20 },
		TipColor = { 160, 80, 10 },
		InputColor = { 60, 35, 8 },
		StrokeTransparency = 1,
	},

	-- 41 obsidian  almost pure black ice-blue highlight
	{
		Name = "obsidian",
		Font = { "JetBrainsMono", "Regular", "Normal" },
		TextSize = 13,
		BackgroundColor = { 4, 4, 8 },
		ForegroundColor = { 8, 8, 14 },
		BorderColor = { 24, 28, 44 },
		UnselectedColor = { 14, 14, 24 },
		SelectedColor = { 100, 180, 255 },
		TextColor = { 200, 210, 230 },
		DimmedTextColor = { 80, 90, 120 },
		ToggleBackgroundColor = { 100, 180, 255 },
		TipColor = { 100, 180, 255 },
		InputColor = { 200, 210, 230 },
		StrokeTransparency = 1,
	},

	-- 42 solarized dark (classic)
	{
		Name = "solarized dark",
		Font = { "Inconsolata", "Regular", "Normal" },
		TextSize = 13,
		BackgroundColor = { 0, 43, 54 },
		ForegroundColor = { 7, 54, 66 },
		BorderColor = { 0, 77, 94 },
		UnselectedColor = { 0, 65, 80 },
		SelectedColor = { 42, 161, 152 },
		TextColor = { 131, 148, 150 },
		DimmedTextColor = { 60, 90, 100 },
		ToggleBackgroundColor = { 42, 161, 152 },
		TipColor = { 133, 153, 0 },
		InputColor = { 131, 148, 150 },
		StrokeTransparency = 1,
	},

	-- 43 tofu  ultra minimal pure white black ink
	{
		Name = "tofu",
		Font = { "SourceSansPro", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 255, 255, 255 },
		ForegroundColor = { 248, 248, 248 },
		BorderColor = { 220, 220, 220 },
		UnselectedColor = { 238, 238, 238 },
		SelectedColor = { 0, 0, 0 },
		TextColor = { 0, 0, 0 },
		DimmedTextColor = { 150, 150, 150 },
		ToggleBackgroundColor = { 0, 0, 0 },
		TipColor = { 80, 80, 80 },
		InputColor = { 0, 0, 0 },
		StrokeTransparency = 1,
	},

	-- 44 autumn  warm browns ochre burnt orange
	{
		Name = "autumn",
		Font = { "Merriweather", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 42, 24, 12 },
		ForegroundColor = { 60, 36, 18 },
		BorderColor = { 130, 80, 40 },
		UnselectedColor = { 80, 48, 24 },
		SelectedColor = { 230, 150, 40 },
		TextColor = { 248, 220, 170 },
		DimmedTextColor = { 180, 130, 80 },
		ToggleBackgroundColor = { 200, 110, 30 },
		TipColor = { 200, 110, 30 },
		InputColor = { 248, 220, 170 },
		StrokeTransparency = 1,
	},

	-- 45 neon green (80s)  truly fluorescent
	{
		Name = "neon 80s",
		Font = { "RobotoMono", "Bold", "Normal" },
		TextSize = 13,
		BackgroundColor = { 0, 0, 0 },
		ForegroundColor = { 6, 6, 6 },
		BorderColor = { 30, 0, 60 },
		UnselectedColor = { 14, 0, 28 },
		SelectedColor = { 0, 255, 100 },
		TextColor = { 0, 255, 100 },
		DimmedTextColor = { 0, 120, 50 },
		ToggleBackgroundColor = { 255, 0, 200 },
		TipColor = { 255, 0, 200 },
		InputColor = { 0, 255, 100 },
		StrokeTransparency = 1,
	},

	-- 46 nord dark  calm cool palette
	{
		Name = "nord dark",
		Font = { "JetBrainsMono", "Regular", "Normal" },
		TextSize = 13,
		BackgroundColor = { 46, 52, 64 },
		ForegroundColor = { 59, 66, 82 },
		BorderColor = { 76, 86, 106 },
		UnselectedColor = { 67, 76, 94 },
		SelectedColor = { 136, 192, 208 },
		TextColor = { 236, 239, 244 },
		DimmedTextColor = { 140, 148, 165 },
		ToggleBackgroundColor = { 163, 190, 140 },
		TipColor = { 180, 142, 173 },
		InputColor = { 236, 239, 244 },
		StrokeTransparency = 1,
	},

	-- 47 gruvbox dark (classic)
	{
		Name = "gruvbox dark",
		Font = { "Inconsolata", "Regular", "Normal" },
		TextSize = 13,
		BackgroundColor = { 40, 40, 40 },
		ForegroundColor = { 50, 48, 47 },
		BorderColor = { 80, 73, 69 },
		UnselectedColor = { 60, 58, 55 },
		SelectedColor = { 184, 187, 38 },
		TextColor = { 235, 219, 178 },
		DimmedTextColor = { 146, 131, 116 },
		ToggleBackgroundColor = { 250, 189, 47 },
		TipColor = { 211, 134, 155 },
		InputColor = { 235, 219, 178 },
		StrokeTransparency = 1,
	},

	-- 48 midnight cherry  dark with cherry-red pop
	{
		Name = "midnight cherry",
		Font = { "Nunito", "Regular", "Normal" },
		TextSize = 14,
		BackgroundColor = { 12, 6, 14 },
		ForegroundColor = { 20, 10, 22 },
		BorderColor = { 66, 18, 50 },
		UnselectedColor = { 36, 12, 36 },
		SelectedColor = { 220, 60, 90 },
		TextColor = { 250, 220, 235 },
		DimmedTextColor = { 160, 90, 130 },
		ToggleBackgroundColor = { 220, 60, 90 },
		TipColor = { 200, 50, 80 },
		InputColor = { 250, 220, 235 },
		StrokeTransparency = 1,
	},

	-- 49 silver fox  light silver dark graphite text
	{
		Name = "silver fox",
		Font = { "Gotham", "Regular", "Normal" },
		TextSize = 13,
		BackgroundColor = { 225, 228, 232 },
		ForegroundColor = { 210, 214, 220 },
		BorderColor = { 180, 185, 194 },
		UnselectedColor = { 196, 200, 208 },
		SelectedColor = { 50, 55, 70 },
		TextColor = { 30, 32, 42 },
		DimmedTextColor = { 130, 135, 150 },
		ToggleBackgroundColor = { 50, 55, 70 },
		TipColor = { 70, 75, 95 },
		InputColor = { 30, 32, 42 },
		StrokeTransparency = 1,
	},

	-- 50 electric blue  deep dark electric cyan-blue accent
	{
		Name = "electric blue",
		Font = { "RobotoMono", "Regular", "Normal" },
		TextSize = 13,
		BackgroundColor = { 6, 8, 22 },
		ForegroundColor = { 10, 14, 36 },
		BorderColor = { 30, 50, 120 },
		UnselectedColor = { 18, 28, 70 },
		SelectedColor = { 40, 140, 255 },
		TextColor = { 190, 215, 255 },
		DimmedTextColor = { 80, 110, 200 },
		ToggleBackgroundColor = { 40, 140, 255 },
		TipColor = { 40, 140, 255 },
		InputColor = { 190, 215, 255 },
		StrokeTransparency = 1,
	},
}

-- community / user-submitted themes
-- these need owner sign-off before they're usable
-- to approve: set the "pending" key to nil (or false) and ship a new version
-- format matches builtins + the extra "pending" / "author" keys

local community: { { [string]: any } } = {
	-- example slot  replace with real submissions
	-- {
	-- 	name = "community example"
	-- 	author = "username"
	-- 	pending = true -- set to nil/false to approve
	-- 	textsize = 14
	-- 	
	-- }
}

--  tab builder

return function(ui: UiLibrary.Ui, plugin: Plugin)
	local tab = ui:CreateTab("Themes")

	-- force layout order so sections appear in insertion order
	local tabContent = tab.instance
	if tabContent then
		local layout = tabContent:FindFirstChildOfClass("UIListLayout")
		if layout then layout.SortOrder = Enum.SortOrder.LayoutOrder end

		if not tabContent:IsA("ScrollingFrame") then
			local sf = Instance.new("ScrollingFrame")
			sf.Name = "ThemesScroll"
			sf.Size = UDim2.new(1, 0, 1, 0)
			sf.Position = UDim2.new(0, 0, 0, 0)
			sf.BackgroundTransparency = 1
			sf.BorderSizePixel = 0
			sf.ScrollBarThickness = 4
			sf.ScrollBarImageTransparency = 0.4
			sf.VerticalScrollBarInset = Enum.ScrollBarInset.ScrollBar
			sf.AutomaticCanvasSize = Enum.AutomaticSize.Y
			sf.CanvasSize = UDim2.new(0, 0, 0, 0)
			sf.ScrollingDirection = Enum.ScrollingDirection.Y
			sf.Parent = tabContent

			local innerLayout = Instance.new("UIListLayout")
			innerLayout.SortOrder = Enum.SortOrder.LayoutOrder
			innerLayout.Padding = UDim.new(0, 2)
			innerLayout.Parent = sf

			if layout then layout:Destroy() end

			local function reparentElements()
				for _, child in tabContent:GetChildren() do
					if child ~= sf and not child:IsA("UIListLayout") and not child:IsA("UIPadding") then
						child.Parent = sf
					end
				end
			end
			reparentElements()

			tabContent.ChildAdded:Connect(function(child)
				if child ~= sf and not child:IsA("UIListLayout") and not child:IsA("UIPadding") then
					child.Parent = sf
				end
			end)

			tabContent = sf
		end
	end

	local function order(widget: any, n: number)
		local inst = widget.instance or widget
		if inst and typeof(inst) == "Instance" then inst.LayoutOrder = n end
	end

	-- active theme label (top of tab)
	local activeLabel = tab:CreateLabel("active: " .. Theme.get().Name)
	order(activeLabel, 1)

	local function refreshActiveLabel()
		local name = Theme.get().Name
		if name == "" then name = "studio" end
		activeLabel:ChangeText("active: " .. name)
	end

	-- built-in themes section
	local builtinLabel = tab:CreateLabel("built-in themes")
	order(builtinLabel, 2)

	-- register all built-ins into the theme system (idempotent  skips dupes by name)
	local builtinIds: { [string]: string } = {}

	local function registerBuiltins()
		local existing = Theme.getThemes()
		for _, raw in builtins do
			local alreadyId: string? = nil
			for id, t in existing do
				if (t :: any).Name == raw.Name then
					alreadyId = id
					break
				end
			end
			if not alreadyId then
				local ok, id = pcall(Theme.add, raw)
				if ok then builtinIds[raw.Name] = id :: string end
			else
				builtinIds[raw.Name] = alreadyId
			end
		end
	end
	registerBuiltins()

	local i = 0
	for _, raw in builtins do
		local themeId = builtinIds[raw.Name]
		if themeId then
			local btn = tab:CreateButton(raw.Name, function()
				local ok, err = pcall(Theme.set, themeId)
				if ok then
					ui:UpdateTheme()
					refreshActiveLabel()
				else
					ui:Notify("Themes", "couldn't apply theme: " .. tostring(err))
				end
			end)
			order(btn, 10 + i)
		end
		i += 1
	end

	-- community themes section
	local communityLabel = tab:CreateLabel("community themes")
	order(communityLabel, 200)

	-- count approved ones so we can show a "none yet" hint
	local approvedCount = 0
	local ci = 0

	for _, raw in community do
		if raw.pending then continue end
		approvedCount += 1

		-- register if not already in
		local commId: string? = nil
		local existing = Theme.getThemes()
		for id, t in existing do
			if (t :: any).Name == raw.Name then commId = id break end
		end
		if not commId then
			local ok, newId = pcall(Theme.add, raw)
			if ok then commId = newId :: string end
		end

		if commId then
			local capturedId = commId :: string
			local btn = tab:CreateButton(raw.Name, function()
				local ok, err = pcall(Theme.set, capturedId)
				if ok then
					ui:UpdateTheme()
					refreshActiveLabel()
				else
					ui:Notify("Themes", "couldn't apply theme: " .. tostring(err))
				end
			end)
			order(btn, 210 + ci)
		end
		ci += 1
	end

	if approvedCount == 0 then
		local noneLabel = tab:CreateLabel("no community themes yet  submit yours below")
		order(noneLabel, 211)
	end

	local customLabel = tab:CreateLabel("custom themes")
	order(customLabel, 400)

	local customHint = tab:CreateLabel("use the themes panel in the re/up desktop app to build and submit custom themes")
	order(customHint, 401)

	-- saved custom themes list
	local savedLabel = tab:CreateLabel("saved custom themes")
	order(savedLabel, 600)

	local function rebuildCustomList()
		local allThemes = Theme.getThemes()
		local builtinNames: { [string]: boolean } = { Studio = true }
		for _, raw in builtins do
			builtinNames[raw.Name] = true
		end

		local n = 0
		for id, raw in allThemes do
			local name: string = (raw :: any).Name or ""
			if builtinNames[name] then continue end
			if (raw :: any).pending then continue end

			n += 1
			local capturedId = id
			local capturedName = name

			local applyBtn = tab:CreateButton(name, function()
				local ok, err = pcall(Theme.set, capturedId)
				if ok then
					ui:UpdateTheme()
					refreshActiveLabel()
				else
					ui:Notify("Themes", "apply failed: " .. tostring(err))
				end
			end)
			order(applyBtn, 610 + (n * 2) - 1)

			local delBtn = tab:CreateButton("delete: " .. name, function()
				if Theme.get().Id == capturedId then
					ui:Notify("Themes", "can't delete the active theme")
					return
				end
				Theme.remove(capturedId)
				ui:Notify("Themes", "deleted: " .. capturedName)
			end)
			order(delBtn, 610 + (n * 2))
		end

		if n == 0 then
			local noneYet = tab:CreateLabel("no saved custom themes yet")
			order(noneYet, 611)
		end
	end
	rebuildCustomList()

	-- studio fallback
	local studioBtn = tab:CreateButton("reset to studio theme", function()
		local ok, err = pcall(Theme.set, "Studio")
		if ok then
			ui:UpdateTheme()
			refreshActiveLabel()
		else
			ui:Notify("Themes", "couldn't reset: " .. tostring(err))
		end
	end)
	order(studioBtn, 700)
end

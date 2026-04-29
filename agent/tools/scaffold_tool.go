package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	agentcontext "github.com/ethen-aiden/code-agent/agent/context"
)

// FrameworkSpec describes the structure and constraints for a supported frontend framework.
type FrameworkSpec struct {
	// Name is the canonical framework identifier ("vue3", "react", "react-native").
	Name string
	// Label is the human-readable name shown in prompts.
	Label string
	// PackageJSON is the content of the initial package.json.
	PackageJSON string
	// ViteConfig is the content of vite.config.ts (empty for React Native).
	ViteConfig string
	// TsConfig is the content of tsconfig.json.
	TsConfig string
	// IndexHTML is the content of index.html (web-only).
	IndexHTML string
	// EntryFile is the main app entry (src/main.ts for Vue, src/index.tsx for React, App.tsx for RN).
	EntryFileName string
	EntryContent  string
	// AppFile is the root component.
	AppFileName string
	AppContent  string
	// ExtraFiles holds additional config files (key = rel path, value = content).
	ExtraFiles map[string]string
}

// frameworkSpecs holds the scaffold templates for each supported framework.
var frameworkSpecs = map[string]*FrameworkSpec{
	"vue3": {
		Name:  "vue3",
		Label: "Vue 3 (Vite + TypeScript + Composition API)",
		PackageJSON: `{
  "name": "vue3-app",
  "version": "0.0.1",
  "private": true,
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc && vite build",
    "preview": "vite preview",
    "type-check": "vue-tsc --noEmit"
  },
  "dependencies": {
    "vue": "^3.4.0"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.0.0",
    "typescript": "^5.3.0",
    "vite": "^5.0.0",
    "vue-tsc": "^1.8.0"
  }
}`,
		ViteConfig: `import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
})
`,
		TsConfig: `{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "module": "ESNext",
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "preserve",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["src/**/*.ts", "src/**/*.tsx", "src/**/*.vue"],
  "exclude": ["node_modules", "dist"]
}
`,
		IndexHTML: `<!DOCTYPE html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Vue 3 App</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
`,
		EntryFileName: "src/main.ts",
		EntryContent: `import { createApp } from 'vue'
import App from './App.vue'

createApp(App).mount('#app')
`,
		AppFileName: "src/App.vue",
		AppContent: `<template>
  <div id="app">
    <h1>Hello Vue 3!</h1>
  </div>
</template>

<script setup lang="ts">
// Composition API setup
</script>

<style scoped>
#app {
  font-family: Avenir, Helvetica, Arial, sans-serif;
  text-align: center;
  color: #2c3e50;
  margin-top: 60px;
}
</style>
`,
	},

	"react": {
		Name:  "react",
		Label: "React 18 (Vite + TypeScript + Tailwind CSS)",
		PackageJSON: `{
  "name": "react-app",
  "version": "0.0.1",
  "private": true,
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "type-check": "tsc --noEmit"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0"
  },
  "devDependencies": {
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0",
    "@vitejs/plugin-react": "^4.0.0",
    "autoprefixer": "^10.4.0",
    "postcss": "^8.4.0",
    "tailwindcss": "^3.4.0",
    "typescript": "^5.3.0",
    "vite": "^5.0.0"
  }
}`,
		ViteConfig: `import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
})
`,
		TsConfig: `{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["src"],
  "exclude": ["node_modules", "dist"]
}
`,
		IndexHTML: `<!DOCTYPE html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>React App</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
`,
		EntryFileName: "src/main.tsx",
		EntryContent: `import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
`,
		AppFileName: "src/App.tsx",
		AppContent: `import React from 'react'

function App(): React.JSX.Element {
  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center">
      <h1 className="text-3xl font-bold text-gray-900">Hello React!</h1>
    </div>
  )
}

export default App
`,
		ExtraFiles: map[string]string{
			"tailwind.config.js": `/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {},
  },
  plugins: [],
}
`,
			"postcss.config.js": `export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
}
`,
			"src/index.css": `@tailwind base;
@tailwind components;
@tailwind utilities;
`,
		},
	},

	"react-native": {
		Name:  "react-native",
		Label: "React Native (Expo + NativeWind + TypeScript)",
		PackageJSON: `{
  "name": "react-native-app",
  "version": "1.0.0",
  "main": "expo-router/entry",
  "scripts": {
    "start": "expo start",
    "android": "expo start --android",
    "ios": "expo start --ios",
    "web": "expo start --web",
    "dev": "expo start --web",
    "type-check": "tsc --noEmit"
  },
  "dependencies": {
    "expo": "~51.0.0",
    "expo-router": "~3.5.0",
    "expo-status-bar": "~1.12.1",
    "nativewind": "^4.0.1",
    "react": "18.2.0",
    "react-native": "0.74.0",
    "react-native-safe-area-context": "4.10.1",
    "react-native-screens": "~3.31.1"
  },
  "devDependencies": {
    "@babel/core": "^7.24.0",
    "@types/react": "~18.2.0",
    "tailwindcss": "^3.4.0",
    "typescript": "^5.3.0"
  }
}`,
		TsConfig: `{
  "extends": "expo/tsconfig.base",
  "compilerOptions": {
    "strict": true,
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["**/*.ts", "**/*.tsx", ".expo/types/**/*.d.ts", "expo-env.d.ts", "nativewind-env.d.ts"]
}
`,
		EntryFileName: "app/_layout.tsx",
		EntryContent: `import { Stack } from 'expo-router'
import '../global.css'

export default function RootLayout() {
  return (
    <Stack screenOptions={{ headerShown: false }}>
      <Stack.Screen name="index" />
    </Stack>
  )
}
`,
		AppFileName: "app/index.tsx",
		AppContent: `import { View, Text } from 'react-native'
import { StatusBar } from 'expo-status-bar'

export default function HomeScreen() {
  return (
    <View className="flex-1 items-center justify-center bg-white">
      <Text className="text-3xl font-bold text-gray-900">Hello React Native!</Text>
      <StatusBar style="auto" />
    </View>
  )
}
`,
		ExtraFiles: map[string]string{
			"babel.config.js": `module.exports = function (api) {
  api.cache(true)
  return {
    presets: [
      ['babel-preset-expo', { jsxImportSource: 'nativewind' }],
    ],
    plugins: ['nativewind/babel'],
  }
}
`,
			"tailwind.config.js": `/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./app/**/*.{js,jsx,ts,tsx}",
    "./src/**/*.{js,jsx,ts,tsx}",
    "./components/**/*.{js,jsx,ts,tsx}",
  ],
  presets: [require('nativewind/preset')],
  theme: {
    extend: {},
  },
  plugins: [],
}
`,
			"global.css": `@tailwind base;
@tailwind components;
@tailwind utilities;
`,
			"nativewind-env.d.ts": `/// <reference types="nativewind/types" />
`,
			"metro.config.js": `const { getDefaultConfig } = require('expo/metro-config')
const { withNativeWind } = require('nativewind/metro')

const config = getDefaultConfig(__dirname)

module.exports = withNativeWind(config, { input: './global.css' })
`,
		},
	},
}

// GetFrameworkSpec returns the framework specification for the given framework name.
// Returns nil if the framework is not supported.
func GetFrameworkSpec(framework string) *FrameworkSpec {
	return frameworkSpecs[framework]
}

// GetFrameworkPromptConstraints returns the system prompt section describing framework-specific
// constraints to be injected into the Planner and Executor system prompts.
func GetFrameworkPromptConstraints(framework string) string {
	spec := frameworkSpecs[framework]
	if spec == nil {
		return ""
	}
	switch framework {
	case "vue3":
		return `## Framework Constraints: Vue 3 (Vite + TypeScript)

You MUST follow these rules for ALL generated code:
- Framework: Vue 3 with Composition API (<script setup lang="ts">). NEVER use Options API.
- Language: TypeScript only. No plain JavaScript files.
- Build tool: Vite. Config file: vite.config.ts.
- Module system: ES Modules (import/export). No CommonJS (require/module.exports).
- File extensions: .vue for components, .ts for logic, .tsx only when JSX is needed.
- Directory structure:
  src/
    components/   (shared reusable components)
    views/ or pages/  (route-level components)
    composables/  (reusable Composition API hooks, prefix with "use")
    stores/       (Pinia stores if state management is needed)
    assets/       (static assets)
    router/       (Vue Router config if routing is needed)
    types/        (TypeScript type definitions)
  App.vue         (root component)
  main.ts         (entry point)
- Type-check command: "vue-tsc --noEmit"
- DO NOT use class components, mixins, or Vue 2 patterns.
- DO NOT use @Options decorator or defineComponent with options object unless strictly necessary.
`
	case "react":
		return `## Framework Constraints: React 18 (Vite + TypeScript)

You MUST follow these rules for ALL generated code:
- Framework: React 18 with functional components and Hooks. NEVER use class components.
- Language: TypeScript only. All components use .tsx extension.
- Build tool: Vite. Config file: vite.config.ts.
- Module system: ES Modules (import/export). No CommonJS.
- File extensions: .tsx for React components, .ts for pure TypeScript.
- Directory structure:
  src/
    components/   (shared reusable components)
    pages/        (route-level components)
    hooks/        (custom React hooks, prefix with "use")
    context/      (React Context providers)
    store/        (state management if needed, e.g. Zustand)
    assets/       (static assets)
    types/        (TypeScript type definitions)
    utils/        (utility functions)
  App.tsx         (root component)
  main.tsx        (entry point)
- Return type annotation for components: React.JSX.Element or React.FC<Props>.
- Type-check command: "tsc --noEmit"
- Props must be typed with an interface or type alias.
- DO NOT use class components, HOCs, or deprecated lifecycle methods.
- DO NOT use React.createClass.
`
	case "react-native":
		return `## Framework Constraints: React Native (Expo + NativeWind + TypeScript)

You MUST follow these rules for ALL generated code:
- Framework: React Native with Expo SDK 51. expo-router for file-based navigation.
- Styling: NativeWind v4 (Tailwind CSS for React Native). Use className="..." on ALL components — NEVER use StyleSheet.create() or inline style={{}} objects. NativeWind is already configured in babel.config.js and global.css.
- Language: TypeScript only. All components use .tsx extension.
- Navigation: expo-router (file-based routing under app/ directory).
- Directory structure:
  app/            (expo-router pages and layouts)
    _layout.tsx   (root layout — already imports global.css)
    index.tsx     (home screen)
  src/
    components/   (shared reusable UI components)
    hooks/        (custom hooks, prefix "use")
    store/        (Zustand or Context if needed)
    types/        (TypeScript type definitions)
    utils/        (utility functions)
- Import UI primitives from 'react-native': View, Text, TouchableOpacity, ScrollView, Pressable, FlatList, Image, TextInput.
- For gestures/interactions: onPress (not onClick). For long press: onLongPress.
- NEVER use: document, window, localStorage, CSS files (except global.css which is already there), <div>, <span>, <p>, <h1> etc.
- Platform-specific: use Platform.OS === 'ios'/'android'/'web' for differences.
- Type-check command: "tsc --noEmit"
- DO NOT use class components.
- ALWAYS import React from 'react' when using JSX hooks.
`
	}
	return ""
}

// ─── Scaffold Tool ────────────────────────────────────────────────────────────

var scaffoldProjectToolInfo = &schema.ToolInfo{
	Name: "scaffold_project",
	Desc: `Initialize a new frontend project with the chosen framework's boilerplate files.
Call this tool ONCE at the very beginning of a new project, before generating any feature code.
Parameters:
- framework: The frontend framework. One of: "vue3", "react", "react-native" (required)

This writes package.json, tsconfig.json, vite.config.ts, index.html, and the root App component
into the current project directory. After scaffolding, install dependencies with npm install.`,
	ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"framework": {
			Type:     schema.String,
			Desc:     `Frontend framework: "vue3", "react", or "react-native"`,
			Required: true,
		},
	}),
}

type ScaffoldProjectInput struct {
	Framework string `json:"framework"`
}

type scaffoldProjectTool struct {
	baseRoot string
}

// NewScaffoldProjectTool creates a tool that writes framework boilerplate files.
func NewScaffoldProjectTool(baseRoot string) tool.InvokableTool {
	return &scaffoldProjectTool{baseRoot: baseRoot}
}

func (s *scaffoldProjectTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return scaffoldProjectToolInfo, nil
}

func (s *scaffoldProjectTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	input := &ScaffoldProjectInput{}
	if err := json.Unmarshal([]byte(argumentsInJSON), input); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	spec := GetFrameworkSpec(input.Framework)
	if spec == nil {
		return fmt.Sprintf("error: unsupported framework %q. Choose one of: vue3, react, react-native", input.Framework), nil
	}

	// Resolve per-project directory
	root := projectDir(ctx, s.baseRoot)
	if err := os.MkdirAll(root, 0755); err != nil {
		return fmt.Sprintf("error: failed to create project directory: %s", err.Error()), nil
	}

	// Store chosen framework in agent context so other tools and prompts can read it
	agentcontext.AppendContextParams(ctx, map[string]interface{}{
		"framework": input.Framework,
	})

	var written []string
	write := func(relPath, content string) error {
		if content == "" {
			return nil
		}
		full := filepath.Join(root, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", relPath, err)
		}
		written = append(written, relPath)
		return nil
	}

	files := map[string]string{
		"package.json":   spec.PackageJSON,
		"tsconfig.json":  spec.TsConfig,
		"vite.config.ts": spec.ViteConfig,
		"index.html":     spec.IndexHTML,
		spec.EntryFileName: spec.EntryContent,
		spec.AppFileName:   spec.AppContent,
	}
	for k, v := range spec.ExtraFiles {
		files[k] = v
	}

	for relPath, content := range files {
		if err := write(relPath, content); err != nil {
			return fmt.Sprintf("error: %s", err.Error()), nil
		}
	}

	return fmt.Sprintf(
		"success: scaffolded %s project at %s\nFiles created: %v\nNext step: run 'npm install' in the project directory.",
		spec.Label, root, written,
	), nil
}

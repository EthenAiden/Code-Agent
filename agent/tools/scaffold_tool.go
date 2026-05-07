package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	agentcontext "github.com/ethen-aiden/code-agent/agent/context"
	"github.com/ethen-aiden/code-agent/prompts"
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
		Label: "React 18 (Vite + TypeScript + Tailwind CSS + shadcn/ui)",
		PackageJSON: `{
  "name": "react-app",
  "version": "0.0.1",
  "private": true,
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview",
    "type-check": "tsc --noEmit"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "react-router-dom": "^6.23.0",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.1.0",
    "tailwind-merge": "^2.3.0",
    "lucide-react": "^0.395.0",
    "@radix-ui/react-slot": "^1.0.2",
    "@radix-ui/react-dialog": "^1.0.5",
    "@radix-ui/react-tabs": "^1.0.4",
    "@radix-ui/react-label": "^2.0.2",
    "@radix-ui/react-separator": "^1.0.3"
  },
  "devDependencies": {
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0",
    "@types/react-router-dom": "^5.3.3",
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
			"tailwind.config.ts": `import type { Config } from 'tailwindcss'

export default {
  content: [
    './index.html',
    './src/**/*.{ts,tsx}',
  ],
  theme: {
    extend: {
      colors: {
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        primary: {
          DEFAULT: 'hsl(var(--primary))',
          foreground: 'hsl(var(--primary-foreground))',
          glow: 'hsl(var(--primary-glow))',
        },
        secondary: {
          DEFAULT: 'hsl(var(--secondary))',
          foreground: 'hsl(var(--secondary-foreground))',
        },
        accent: {
          DEFAULT: 'hsl(var(--accent))',
          foreground: 'hsl(var(--accent-foreground))',
        },
        muted: {
          DEFAULT: 'hsl(var(--muted))',
          foreground: 'hsl(var(--muted-foreground))',
        },
        card: {
          DEFAULT: 'hsl(var(--card))',
          foreground: 'hsl(var(--card-foreground))',
        },
        border: 'hsl(var(--border))',
        input: 'hsl(var(--input))',
        ring: 'hsl(var(--ring))',
      },
      borderRadius: {
        lg: 'var(--radius)',
        md: 'calc(var(--radius) - 2px)',
        sm: 'calc(var(--radius) - 4px)',
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
      },
    },
  },
  plugins: [],
} satisfies Config
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

@layer base {
  :root {
    --background: 0 0% 100%;
    --foreground: 222 47% 11%;
    --primary: 262 83% 58%;
    --primary-foreground: 0 0% 100%;
    --primary-glow: 270 70% 75%;
    --secondary: 210 40% 96%;
    --secondary-foreground: 222 47% 11%;
    --accent: 316 70% 58%;
    --accent-foreground: 0 0% 100%;
    --muted: 210 40% 96%;
    --muted-foreground: 215 16% 47%;
    --card: 0 0% 100%;
    --card-foreground: 222 47% 11%;
    --border: 214 32% 91%;
    --input: 214 32% 91%;
    --ring: 262 83% 58%;
    --radius: 0.5rem;
    --gradient-primary: linear-gradient(135deg, hsl(var(--primary)), hsl(var(--accent)));
    --gradient-subtle: linear-gradient(180deg, hsl(var(--background)), hsl(var(--muted)));
    --gradient-hero: linear-gradient(135deg, hsl(var(--primary) / 0.12), hsl(var(--accent) / 0.08));
    --shadow-elegant: 0 10px 30px -10px hsl(var(--primary) / 0.3);
    --shadow-glow: 0 0 40px hsl(var(--primary-glow) / 0.4);
    --shadow-card: 0 4px 20px -4px hsl(var(--foreground) / 0.08);
    --transition-smooth: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    --transition-bounce: all 0.4s cubic-bezier(0.34, 1.56, 0.64, 1);
  }
}

@layer utilities {
  .gradient-primary { background: var(--gradient-primary); }
  .gradient-subtle { background: var(--gradient-subtle); }
  .gradient-hero { background: var(--gradient-hero); }
  .shadow-elegant { box-shadow: var(--shadow-elegant); }
  .shadow-glow { box-shadow: var(--shadow-glow); }
  .shadow-card { box-shadow: var(--shadow-card); }
  .transition-smooth { transition: var(--transition-smooth); }
  .transition-bounce { transition: var(--transition-bounce); }
}
`,
			"src/lib/utils.ts": `import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
`,
			"src/components/ui/button.tsx": `import * as React from 'react'
import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const buttonVariants = cva(
  'inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium transition-smooth focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50',
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground shadow hover:opacity-90',
        secondary: 'bg-secondary text-secondary-foreground hover:bg-secondary/80',
        outline: 'border border-border bg-background hover:bg-muted hover:text-foreground',
        ghost: 'hover:bg-muted hover:text-foreground',
        link: 'text-primary underline-offset-4 hover:underline',
        hero: 'gradient-primary text-primary-foreground shadow-elegant hover:shadow-glow transition-bounce',
      },
      size: {
        default: 'h-9 px-4 py-2',
        sm: 'h-8 rounded-md px-3 text-xs',
        lg: 'h-11 rounded-md px-8 text-base',
        xl: 'h-14 rounded-lg px-10 text-lg',
        icon: 'h-9 w-9',
      },
    },
    defaultVariants: { variant: 'default', size: 'default' },
  }
)

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : 'button'
    return <Comp className={cn(buttonVariants({ variant, size, className }))} ref={ref} {...props} />
  }
)
Button.displayName = 'Button'

export { Button, buttonVariants }
`,
			"src/components/ui/card.tsx": `import * as React from 'react'
import { cn } from '@/lib/utils'

const Card = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn('rounded-lg border border-border bg-card text-card-foreground shadow-card', className)} {...props} />
  )
)
Card.displayName = 'Card'

const CardHeader = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn('flex flex-col space-y-1.5 p-6', className)} {...props} />
  )
)
CardHeader.displayName = 'CardHeader'

const CardTitle = React.forwardRef<HTMLParagraphElement, React.HTMLAttributes<HTMLHeadingElement>>(
  ({ className, ...props }, ref) => (
    <h3 ref={ref} className={cn('font-semibold leading-none tracking-tight text-foreground', className)} {...props} />
  )
)
CardTitle.displayName = 'CardTitle'

const CardDescription = React.forwardRef<HTMLParagraphElement, React.HTMLAttributes<HTMLParagraphElement>>(
  ({ className, ...props }, ref) => (
    <p ref={ref} className={cn('text-sm text-muted-foreground', className)} {...props} />
  )
)
CardDescription.displayName = 'CardDescription'

const CardContent = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn('p-6 pt-0', className)} {...props} />
  )
)
CardContent.displayName = 'CardContent'

const CardFooter = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn('flex items-center p-6 pt-0', className)} {...props} />
  )
)
CardFooter.displayName = 'CardFooter'

export { Card, CardHeader, CardFooter, CardTitle, CardDescription, CardContent }
`,
			"src/components/ui/input.tsx": `import * as React from 'react'
import { cn } from '@/lib/utils'

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {}

const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type, ...props }, ref) => (
    <input
      type={type}
      className={cn(
        'flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm text-foreground shadow-sm transition-smooth placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50',
        className
      )}
      ref={ref}
      {...props}
    />
  )
)
Input.displayName = 'Input'

export { Input }
`,
			"src/components/ui/badge.tsx": `import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const badgeVariants = cva(
  'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-smooth',
  {
    variants: {
      variant: {
        default: 'border-transparent bg-primary text-primary-foreground',
        secondary: 'border-transparent bg-secondary text-secondary-foreground',
        accent: 'border-transparent bg-accent text-accent-foreground',
        outline: 'border-border text-foreground',
      },
    },
    defaultVariants: { variant: 'default' },
  }
)

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />
}

export { Badge, badgeVariants }
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
	switch framework {
	case "vue3":
		return prompts.Load("framework_vue3.txt")
	case "react":
		return prompts.Load("framework_react.txt")
	case "react-native":
		return prompts.Load("framework_react_native.txt")
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

	installOut := runNpmInstall(root)
	return fmt.Sprintf(
		"success: scaffolded %s project at %s\nFiles created: %v\nInstalling dependencies...\n%s",
		spec.Label, root, written, installOut,
	), nil
}

// runNpmInstall runs "npm install --prefer-offline" in dir and returns a short status string.
func runNpmInstall(dir string) string {
	npmBin := "npm"
	if runtime.GOOS == "windows" {
		npmBin = "npm.cmd"
	}
	cmd := exec.Command(npmBin, "install", "--prefer-offline")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("npm install failed: %v\n%s", err, decodeOutput(out))
	}
	return "npm install completed"
}

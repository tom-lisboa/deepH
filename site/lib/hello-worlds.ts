export type HelloWorldCategory = "cli" | "backend" | "frontend" | "mobile";
export type HelloWorldStyle = "plain" | "tailwind" | "shadcn";

export type HelloWorldEntry = {
  id: string;
  title: string;
  language: string;
  framework?: string;
  category: HelloWorldCategory;
  style: HelloWorldStyle;
  summary: string;
  code: string;
  files: string[];
  deephPrompt: string;
  deephSpec?: string;
  expectedOutput: string;
};

export const helloWorldEntries: HelloWorldEntry[] = [
  {
    id: "bash-cli",
    title: "Bash Hello World",
    language: "Bash",
    category: "cli",
    style: "plain",
    summary: "Script shell mínimo para validar ambiente e pipeline de geração.",
    code: `#!/usr/bin/env bash
echo "Hello, world!"`,
    files: ["hello.sh"],
    deephPrompt: "crie um hello world em Bash com script executável e comentário curto",
    expectedOutput: `./hello.sh -> Hello, world!`
  },
  {
    id: "python-cli",
    title: "Python Hello World",
    language: "Python",
    category: "cli",
    style: "plain",
    summary: "Exemplo básico de arquivo Python para onboarding de codegen.",
    code: `print("Hello, world!")`,
    files: ["hello.py"],
    deephPrompt: "gere um hello world em Python (hello.py) e explique como rodar",
    expectedOutput: `python hello.py -> Hello, world!`
  },
  {
    id: "js-node-cli",
    title: "Node.js Hello World",
    language: "JavaScript",
    framework: "Node.js",
    category: "cli",
    style: "plain",
    summary: "Hello World para Node sem framework, útil para smoke de runtime.",
    code: `console.log("Hello, world!");`,
    files: ["hello.js"],
    deephPrompt: "gere hello world em Node.js usando hello.js",
    expectedOutput: `node hello.js -> Hello, world!`
  },
  {
    id: "ts-node-cli",
    title: "TypeScript Hello World",
    language: "TypeScript",
    framework: "Node.js",
    category: "cli",
    style: "plain",
    summary: "Versão TS com tipagem simples e script de execução sugerido.",
    code: `const message: string = "Hello, world!";
console.log(message);`,
    files: ["hello.ts"],
    deephPrompt: "gere hello world em TypeScript (hello.ts) com tipagem explícita",
    expectedOutput: `ts-node hello.ts -> Hello, world!`
  },
  {
    id: "go-cli",
    title: "Go Hello World",
    language: "Go",
    category: "cli",
    style: "plain",
    summary: "Hello World idiomático em Go com package main.",
    code: `package main

import "fmt"

func main() {
	fmt.Println("Hello, world!")
}`,
    files: ["main.go"],
    deephPrompt: "gere hello world em Go (main.go) idiomático",
    expectedOutput: `go run . -> Hello, world!`
  },
  {
    id: "rust-cli",
    title: "Rust Hello World",
    language: "Rust",
    category: "cli",
    style: "plain",
    summary: "Hello World em Rust para benchmark de geração e revisão.",
    code: `fn main() {
    println!("Hello, world!");
}`,
    files: ["src/main.rs"],
    deephPrompt: "gere hello world em Rust (src/main.rs)",
    expectedOutput: `cargo run -> Hello, world!`
  },
  {
    id: "java-cli",
    title: "Java Hello World",
    language: "Java",
    category: "cli",
    style: "plain",
    summary: "Classe Java mínima para compilação e execução.",
    code: `public class HelloWorld {
  public static void main(String[] args) {
    System.out.println("Hello, world!");
  }
}`,
    files: ["HelloWorld.java"],
    deephPrompt: "gere hello world em Java com classe HelloWorld",
    expectedOutput: `javac HelloWorld.java && java HelloWorld -> Hello, world!`
  },
  {
    id: "c-cli",
    title: "C Hello World",
    language: "C",
    category: "cli",
    style: "plain",
    summary: "Programa C mínimo com stdio.",
    code: `#include <stdio.h>

int main(void) {
  printf("Hello, world!\\n");
  return 0;
}`,
    files: ["main.c"],
    deephPrompt: "gere hello world em C (main.c)",
    expectedOutput: `cc main.c -o hello && ./hello -> Hello, world!`
  },
  {
    id: "cpp-cli",
    title: "C++ Hello World",
    language: "C++",
    category: "cli",
    style: "plain",
    summary: "Programa C++ mínimo com iostream.",
    code: `#include <iostream>

int main() {
  std::cout << "Hello, world!" << std::endl;
  return 0;
}`,
    files: ["main.cpp"],
    deephPrompt: "gere hello world em C++ (main.cpp)",
    expectedOutput: `c++ main.cpp -o hello && ./hello -> Hello, world!`
  },
  {
    id: "csharp-cli",
    title: "C# Hello World",
    language: "C#",
    category: "cli",
    style: "plain",
    summary: "Console app mínimo em C#.",
    code: `Console.WriteLine("Hello, world!");`,
    files: ["Program.cs"],
    deephPrompt: "gere hello world em C# para console (Program.cs)",
    expectedOutput: `dotnet run -> Hello, world!`
  },
  {
    id: "ruby-cli",
    title: "Ruby Hello World",
    language: "Ruby",
    category: "cli",
    style: "plain",
    summary: "Script Ruby simples para quick learning.",
    code: `puts "Hello, world!"`,
    files: ["hello.rb"],
    deephPrompt: "gere hello world em Ruby (hello.rb)",
    expectedOutput: `ruby hello.rb -> Hello, world!`
  },
  {
    id: "php-cli",
    title: "PHP Hello World",
    language: "PHP",
    category: "cli",
    style: "plain",
    summary: "Hello World em PHP executado via CLI.",
    code: `<?php
echo "Hello, world!\\n";`,
    files: ["hello.php"],
    deephPrompt: "gere hello world em PHP para CLI",
    expectedOutput: `php hello.php -> Hello, world!`
  },
  {
    id: "kotlin-cli",
    title: "Kotlin Hello World",
    language: "Kotlin",
    category: "cli",
    style: "plain",
    summary: "Main function simples em Kotlin.",
    code: `fun main() {
    println("Hello, world!")
}`,
    files: ["Main.kt"],
    deephPrompt: "gere hello world em Kotlin (Main.kt)",
    expectedOutput: `kotlinc Main.kt -include-runtime -d app.jar && java -jar app.jar`
  },
  {
    id: "swift-cli",
    title: "Swift Hello World",
    language: "Swift",
    category: "cli",
    style: "plain",
    summary: "Hello World em Swift para script/CLI.",
    code: `print("Hello, world!")`,
    files: ["main.swift"],
    deephPrompt: "gere hello world em Swift (main.swift)",
    expectedOutput: `swift main.swift -> Hello, world!`
  },
  {
    id: "fastapi-backend",
    title: "FastAPI Hello World API",
    language: "Python",
    framework: "FastAPI",
    category: "backend",
    style: "plain",
    summary: "API HTTP simples com rota GET / retornando JSON.",
    code: `from fastapi import FastAPI

app = FastAPI()

@app.get("/")
def hello():
    return {"message": "Hello, world!"}`,
    files: ["main.py"],
    deephPrompt: "gere uma API hello world em FastAPI com rota GET / retornando JSON",
    expectedOutput: `GET / -> {"message":"Hello, world!"}`
  },
  {
    id: "express-backend",
    title: "Express Hello World API",
    language: "JavaScript",
    framework: "Express",
    category: "backend",
    style: "plain",
    summary: "API Express mínima para benchmarking de estrutura web.",
    code: `import express from "express";

const app = express();
app.get("/", (_req, res) => res.json({ message: "Hello, world!" }));
app.listen(3000);`,
    files: ["server.js"],
    deephPrompt: "gere uma API hello world em Express retornando JSON em /",
    expectedOutput: `GET http://localhost:3000/ -> {"message":"Hello, world!"}`
  },
  {
    id: "go-http-backend",
    title: "Go net/http Hello World API",
    language: "Go",
    framework: "net/http",
    category: "backend",
    style: "plain",
    summary: "Servidor Go com handler simples retornando texto.",
    code: `package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "Hello, world!")
	})
	http.ListenAndServe(":8080", nil)
}`,
    files: ["main.go"],
    deephPrompt: "gere uma API hello world em Go usando net/http",
    expectedOutput: `GET http://localhost:8080/ -> Hello, world!`
  },
  {
    id: "next-plain-frontend",
    title: "Next.js Hello World (plain)",
    language: "TypeScript",
    framework: "Next.js App Router",
    category: "frontend",
    style: "plain",
    summary: "Página simples em Next sem Tailwind/shadcn, útil para base mínima.",
    code: `export default function Page() {
  return (
    <main>
      <h1>Hello, world!</h1>
      <p>Powered by deepH</p>
    </main>
  );
}`,
    files: ["app/page.tsx"],
    deephPrompt: "crie uma página hello world em Next.js App Router sem Tailwind",
    expectedOutput: `Página renderiza título e subtítulo simples`
  },
  {
    id: "react-tailwind-frontend",
    title: "React + Tailwind Hello World",
    language: "TypeScript",
    framework: "React",
    category: "frontend",
    style: "tailwind",
    summary: "Componente visual com Tailwind para onboarding de styling.",
    code: `export function HelloCard() {
  return (
    <div className="min-h-screen grid place-items-center bg-slate-950 text-slate-100">
      <div className="rounded-2xl border border-slate-700 bg-slate-900/80 p-8 shadow-2xl">
        <p className="text-sm text-emerald-300">deepH demo</p>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight">Hello, world!</h1>
      </div>
    </div>
  );
}`,
    files: ["src/HelloCard.tsx"],
    deephPrompt: "gere um hello world em React com Tailwind e card elegante",
    expectedOutput: `Tela centralizada com card estilizado em Tailwind`
  },
  {
    id: "next-tailwind-frontend",
    title: "Next.js + Tailwind Hello World",
    language: "TypeScript",
    framework: "Next.js App Router",
    category: "frontend",
    style: "tailwind",
    summary: "Hello World com visual intencional usando Tailwind.",
    code: `export default function Page() {
  return (
    <main className="min-h-screen bg-zinc-950 text-zinc-50 grid place-items-center p-6">
      <section className="w-full max-w-xl rounded-3xl border border-zinc-800 bg-zinc-900 p-8 shadow-[0_20px_80px_rgba(0,0,0,0.45)]">
        <p className="text-xs uppercase tracking-[0.18em] text-emerald-300">deepH starter</p>
        <h1 className="mt-3 text-4xl font-semibold tracking-tight">Hello, world!</h1>
        <p className="mt-2 text-zinc-400">Next.js + Tailwind UI generated via deepH workflow.</p>
      </section>
    </main>
  );
}`,
    files: ["app/page.tsx"],
    deephPrompt: "crie uma página hello world em Next.js com Tailwind e visual forte",
    expectedOutput: `Página Next estilizada com Tailwind e card central`
  },
  {
    id: "next-shadcn-frontend",
    title: "Next.js + shadcn/ui Hello World",
    language: "TypeScript",
    framework: "Next.js + shadcn/ui",
    category: "frontend",
    style: "shadcn",
    summary: "Exemplo usando `Card` e `Button` do shadcn/ui (assume setup pronto).",
    code: `import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function Page() {
  return (
    <main className="min-h-screen grid place-items-center bg-background p-6">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle>Hello, world!</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <p className="text-sm text-muted-foreground">
            Next.js + shadcn/ui generated with deepH.
          </p>
          <Button>Get Started</Button>
        </CardContent>
      </Card>
    </main>
  );
}`,
    files: ["app/page.tsx", "components/ui/button.tsx", "components/ui/card.tsx"],
    deephPrompt:
      "crie uma página hello world em Next.js usando shadcn/ui Card e Button (assuma shadcn já instalado)",
    expectedOutput: `UI com Card + Button do shadcn/ui`
  },
  {
    id: "vue-tailwind-frontend",
    title: "Vue + Tailwind Hello World",
    language: "TypeScript",
    framework: "Vue",
    category: "frontend",
    style: "tailwind",
    summary: "Componente Vue SFC com Tailwind para catálogo visual.",
    code: `<template>
  <main class="min-h-screen grid place-items-center bg-slate-950 text-white p-6">
    <section class="rounded-2xl border border-slate-700 bg-slate-900 px-6 py-5 shadow-xl">
      <p class="text-xs uppercase tracking-[0.2em] text-emerald-300">deepH</p>
      <h1 class="mt-2 text-3xl font-semibold">Hello, world!</h1>
    </section>
  </main>
</template>`,
    files: ["src/App.vue"],
    deephPrompt: "gere hello world em Vue com Tailwind e layout centralizado",
    expectedOutput: `Tela Vue com card estilizado via Tailwind`
  },
  {
    id: "svelte-tailwind-frontend",
    title: "Svelte + Tailwind Hello World",
    language: "TypeScript",
    framework: "Svelte",
    category: "frontend",
    style: "tailwind",
    summary: "Componente Svelte com Tailwind para onboarding visual.",
    code: `<main class="min-h-screen grid place-items-center bg-neutral-950 text-neutral-50 p-6">
  <div class="rounded-2xl border border-neutral-800 bg-neutral-900 p-8 shadow-2xl">
    <p class="text-xs uppercase tracking-[0.2em] text-amber-300">deepH</p>
    <h1 class="mt-2 text-3xl font-semibold">Hello, world!</h1>
  </div>
</main>`,
    files: ["src/routes/+page.svelte"],
    deephPrompt: "gere hello world em Svelte com Tailwind e card central",
    expectedOutput: `Página Svelte estilizada com Tailwind`
  },
  {
    id: "react-native-mobile",
    title: "React Native Hello World",
    language: "TypeScript",
    framework: "React Native",
    category: "mobile",
    style: "plain",
    summary: "Tela inicial simples para onboarding mobile.",
    code: `import { SafeAreaView, Text, View } from "react-native";

export default function App() {
  return (
    <SafeAreaView>
      <View style={{ minHeight: "100%", alignItems: "center", justifyContent: "center" }}>
        <Text>Hello, world!</Text>
      </View>
    </SafeAreaView>
  );
}`,
    files: ["App.tsx"],
    deephPrompt: "gere hello world em React Native com layout centralizado",
    expectedOutput: `Tela mobile com texto centralizado`
  }
];

export const helloWorldCategoryLabels: Record<HelloWorldCategory | "all", string> = {
  all: "Todos",
  cli: "CLI",
  backend: "Backend/API",
  frontend: "Frontend/UI",
  mobile: "Mobile"
};

export const helloWorldStyleLabels: Record<HelloWorldStyle | "all", string> = {
  all: "Todos estilos",
  plain: "Plain",
  tailwind: "Tailwind",
  shadcn: "shadcn/ui"
};

export function helloWorldLanguages() {
  return Array.from(new Set(helloWorldEntries.map((item) => item.language))).sort((a, b) =>
    a.localeCompare(b)
  );
}

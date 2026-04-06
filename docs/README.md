# Docs Development

This directory contains the VitePress documentation site for Pruvon.

## Requirements

- Node.js 20+ recommended
- npm

## Install Dependencies

From the repository root:

```bash
npm run docs:install
```

Or from the `docs/` directory:

```bash
npm install
```

## Run Locally

From the repository root:

```bash
npm run docs:dev
```

Or from the `docs/` directory, start the local development server:

```bash
npm run dev
```

By default, VitePress prints the local URL in the terminal, usually `http://localhost:5173`.

## Build The Site

From the repository root:

```bash
npm run build
```

Or from the `docs/` directory:

```bash
npm run build
```

The generated static site is written to:

```text
.vitepress/dist
```

When building from the repository root for CI or Cloudflare, the output path is `docs/.vitepress/dist`.

## Preview The Production Build

From the repository root:

```bash
npm run docs:preview
```

Or from the `docs/` directory, preview the generated site locally:

```bash
npm run preview
```

## Project Structure

- `index.md` is the docs home page
- `.vitepress/config.mts` contains the VitePress site configuration
- `*.md` files in this directory are the documentation pages

## Common Workflow

1. Run `npm run docs:install` once from the repository root.
2. Run `npm run docs:dev` while editing markdown files.
3. Run `npm run build` before publishing or validating the static output.

## Cloudflare Pages

Recommended settings if the repository root is used as the project root:

- Build command: `npm run build`
- Build output directory: `docs/.vitepress/dist`

Alternative settings if you set `docs/` as the Pages root directory:

- Build command: `npm run build`
- Build output directory: `.vitepress/dist`

## Cloudflare Workers

If you deploy the built static output with Wrangler or a Worker-based static setup, build first with:

```bash
npm run build
```

Then publish the generated files from `docs/.vitepress/dist`.

## Current Pages

- `install.md`
- `configuration.md`
- `security.md`
- `behind-proxy.md`

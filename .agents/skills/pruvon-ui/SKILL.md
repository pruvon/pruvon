---
name: pruvon-ui
description: Use when working on UI, Tailwind, layouts, components, pages, or any visual styling in the Pruvon/Dokkum codebase. This skill is the authoritative design system for typography, color, spacing, surfaces, and component styling decisions across the interface.
---

# PRUVON UI Design System Skill

Instruction for all coding agents:

- Read this file before any UI, layout, component, Tailwind, or visual styling change.
- This file is the single source of truth for the Pruvon/Dokkum UI.
- Do not skip or reinterpret rules without explicit user instruction.
- If this file conflicts with a local preference, this file wins for UI work.

## 1. Typography

### Font Families

| Role | Font | Usage |
|---|---|---|
| **UI Font** | `DM Sans` | All interface text |
| **Serif Font** | `Instrument Serif` | Headings and display text |
| **Mono Font** | `DM Mono` | Technical values: hostname, IP, container ID, URL, env var values, log lines, version strings |

No other font may be introduced. Only `sans-serif`, `serif`, and `monospace` may be used as fallbacks.

### Google Fonts Import

```html
<link href="https://fonts.googleapis.com/css2?family=Instrument+Serif:ital@0;1&family=DM+Mono:wght@400;500&family=DM+Sans:wght@300;400;500&display=swap" rel="stylesheet">
```

### Tailwind fontFamily Config

```js
fontFamily: {
  sans: ['DM Sans', 'sans-serif'],
  serif: ['Instrument Serif', 'serif'],
  mono: ['DM Mono', 'monospace'],
},
```

### Type Scale

| Role | Size | Weight | Line Height | Usage |
|---|---|---|---|---|
| `page-title` | 18px / `text-lg` | 500 | 1.4 | Page H1 |
| `section-title` | 14px / `text-sm` | 500 | 1.4 | Section titles |
| `label` | 12px / `text-xs` | 500 | 1.4 | Form labels, table headers |
| `body` | 14px / `text-sm` | 300 | 1.6 | General body copy |
| `secondary` | 12px / `text-xs` | 300 | 1.5 | Helper text, timestamps |
| `mono-value` | 13px | 400 | 1.5 | Technical values |
| `mono-label` | 12px | 400 | 1.4 | Mono badge/tag |

Allowed font weights are only `300`, `400`, and `500`.

## 2. Color System

### Surfaces

| Token | Hex | Usage |
|---|---|---|
| `surface` | `#F4F6F8` | Page background |
| `sidebar` | `#162436` | Sidebar and dark panel background |
| `card` | `#FFFFFF` | Card and panel surface |
| `input` | `#FFFFFF` | Form input background |
| `dropdown` | `#FFFFFF` | Dropdown background, never default white or gray |

### Sidebar Text

| Token | Hex | Usage |
|---|---|---|
| `sidebar-text` | `#F4F6F8` | Primary text in sidebar (dark bg) |
| `sidebar-text-muted` | `#7A96B0` | Secondary/muted text in sidebar |

### Borders

| Token | Hex | Usage |
|---|---|---|
| `border` (DEFAULT) | `rgba(0,0,0,0.07)` | All borders |
| `border-subtle` | `rgba(0,0,0,0.05)` | Dividers and table row separators |
| `border-strong` | `rgba(0,0,0,0.10)` | Focused inputs and hover states |

### Text

| Token | Hex | Usage |
|---|---|---|
| `text-primary` | `#1A2B3C` | Primary text |
| `text-secondary` | `#3D5166` | Secondary text and labels |
| `text-tertiary` | `#7A96B0` | Placeholders and disabled text |
| `text-link` | `#2D9B6F` | Link color, never blue |

### Brand

| Token | Hex | Usage |
|---|---|---|
| `brand-primary` | `#2D9B6F` | Primary actions and active state |
| `brand-hover` | `#3DBF8A` | Primary hover |
| `brand-subtle` | `rgba(45,155,111,0.10)` | Subtle primary background |
| `brand-accent` | `#3DBF8A` | Light green accent |
| `brand-accent-subtle` | `rgba(61,191,138,0.10)` | Soft accent background |

### Status Colors

| Token | Hex | Token (bg) | Hex (bg) | Usage |
|---|---|---|---|---|
| `status-success` | `#2D9B6F` | `status-success-bg` | `rgba(45,155,111,0.10)` | Running, healthy |
| `status-warning` | `#B06A2B` | `status-warning-bg` | `#FBF0E4` | Degraded, pending |
| `status-danger` | `#9C3F33` | `status-danger-bg` | `#F7EDED` | Stopped, error, destructive |
| `status-neutral` | `#7A96B0` | `status-neutral-bg` | `rgba(0,0,0,0.04)` | Unknown, inactive |

## 3. Tailwind Config Extension

```js
module.exports = {
  theme: {
    extend: {
      fontFamily: {
        sans: ['DM Sans', 'sans-serif'],
        serif: ['Instrument Serif', 'serif'],
        mono: ['DM Mono', 'monospace'],
      },
      fontSize: {
        '2xs': ['11px', { lineHeight: '1.4' }],
      },
      colors: {
        surface: '#F4F6F8',
        sidebar: '#162436',
        'sidebar-text': '#F4F6F8',
        'sidebar-text-muted': '#7A96B0',
        card: '#FFFFFF',
        border: {
          DEFAULT: 'rgba(0,0,0,0.07)',
          subtle: 'rgba(0,0,0,0.05)',
          strong: 'rgba(0,0,0,0.10)',
        },
        brand: {
          primary: '#2D9B6F',
          hover: '#3DBF8A',
          subtle: 'rgba(45,155,111,0.10)',
          accent: '#3DBF8A',
          'accent-subtle': 'rgba(61,191,138,0.10)',
        },
        status: {
          success: '#2D9B6F',
          'success-bg': 'rgba(45,155,111,0.10)',
          warning: '#B06A2B',
          'warning-bg': '#FBF0E4',
          danger: '#9C3F33',
          'danger-bg': '#F7EDED',
          neutral: '#7A96B0',
          'neutral-bg': 'rgba(0,0,0,0.04)',
        },
        text: {
          primary: '#1A2B3C',
          secondary: '#3D5166',
          tertiary: '#7A96B0',
          link: '#2D9B6F',
        },
      },
      borderRadius: {
        none: '0',
        sm: '3px',
        DEFAULT: '5px',
        md: '6px',
        lg: '8px',
        xl: '12px',
        full: '9999px',
      },
      boxShadow: {
        none: 'none',
        dropdown: '0 2px 8px rgba(0,0,0,0.08)',
        modal: '0 8px 32px rgba(0,0,0,0.12)',
        tooltip: '0 2px 6px rgba(0,0,0,0.10)',
      },
    },
  },
}
```

## 4. Spacing System

All spacing must use 4px multiples.

| px | Tailwind |
|---|---|
| 4px | `p-1` / `gap-1` |
| 8px | `p-2` / `gap-2` |
| 12px | `p-3` / `gap-3` |
| 16px | `p-4` / `gap-4` |
| 20px | `p-5` / `gap-5` |
| 24px | `p-6` / `gap-6` |
| 32px | `p-8` / `gap-8` |

### Layout Regions

| Area | Padding |
|---|---|
| Page content area | `px-6 py-5` |
| Card content | `p-4` |
| Section | `py-4` |
| Table cell | `px-4 py-2.5` |
| Form group spacing | `gap-5` |
| Inline field spacing | `gap-3` |

## 5. Border Radius

| Context | Value | Tailwind |
|---|---|---|
| Card, panel | 6px | `rounded-md` |
| Button | 5px | `rounded` |
| Input | 5px | `rounded` |
| Badge / tag | 4px | `rounded-sm` |
| Dropdown | 6px | `rounded-md` |
| Modal | 8px | `rounded-lg` |
| Tooltip | 4px | `rounded-sm` |

## 6. Shadow System

Depth should be expressed by border and surface contrast, not heavy shadow.

| Context | Value | Tailwind |
|---|---|---|
| Card | none, border only | - |
| Dropdown | `0 2px 8px rgba(0,0,0,0.08)` | `shadow-dropdown` |
| Modal | `0 8px 32px rgba(0,0,0,0.12)` | `shadow-modal` |
| Tooltip | `0 2px 6px rgba(0,0,0,0.10)` | `shadow-tooltip` |
| Focus input | `ring-1 ring-brand-primary` | `focus:ring-1 focus:ring-brand-primary` |

`shadow-lg`, `shadow-xl`, and gradients are forbidden.

## 7. Component Anatomy

### 7.1 Buttons

#### Primary

```html
<button class="inline-flex items-center gap-1.5 h-8 px-3
  bg-brand-primary hover:bg-brand-hover
  text-white text-sm font-medium
  rounded border border-brand-primary
  transition-colors duration-150">
  Label
</button>
```

#### Secondary

```html
<button class="inline-flex items-center gap-1.5 h-8 px-3
  bg-card hover:bg-surface
  text-text-primary text-sm font-medium
  rounded border border-border
  transition-colors duration-150">
  Label
</button>
```

#### Tertiary / Ghost

```html
<button class="inline-flex items-center gap-1.5 h-8 px-3
  bg-transparent hover:bg-surface
  text-text-secondary hover:text-text-primary text-sm font-medium
  rounded transition-colors duration-150">
  Label
</button>
```

#### Destructive

```html
<button class="inline-flex items-center gap-1.5 h-8 px-3
  bg-status-danger hover:bg-[#833530]
  text-white text-sm font-medium
  rounded border border-status-danger
  transition-colors duration-150">
  Delete
</button>
```

#### Icon Button

```html
<button class="inline-flex items-center justify-center
  w-7 h-7 rounded
  text-text-secondary hover:text-text-primary hover:bg-surface
  transition-colors duration-150">
  <!-- Lucide icon size={14} -->
</button>
```

Button rules:

- Never use gradients.
- Colored icon tiles are forbidden.
- Standard height is `h-8` (32px), compact is `h-7` (28px).
- Button icons must be 14px.
- Maximum one primary button per page or section.
- `bg-gray-*` buttons are forbidden.

### 7.2 Form Fields

```html
<div class="flex flex-col gap-1.5">
  <label class="text-xs font-medium text-text-secondary">Field Label</label>
  <input
    class="h-8 px-3 bg-card border border-border rounded
      text-sm font-sans text-text-primary
      placeholder:text-text-tertiary
      focus:outline-none focus:ring-1 focus:ring-brand-primary focus:border-brand-primary
      transition-colors duration-150"
  />
  <p class="text-xs text-text-tertiary">Helper text</p>
</div>
```

Technical / mono input for env vars, tokens, and URLs:

```html
<input class="... font-mono text-[13px]" />
```

### 7.3 Dropdown / Select

Dropdown background must be `bg-card`, never default white or system gray.

**Never use `<template x-for>` inside `<select>` elements.** Browser form-filling extensions (LastPass, etc.) can break parsing, causing options to not render. Use a custom dropdown with a button trigger and `x-for` in a `div` container instead.

```html
<div class="relative">
  <!-- Trigger button -->
  <button type="button" @click="dropdownOpen = !dropdownOpen"
    class="inline-flex items-center justify-between h-8 px-3 w-full
      bg-card border border-border rounded
      text-sm text-text-primary
      hover:border-border-strong
      focus:outline-none focus:ring-1 focus:ring-brand-primary
      transition-colors duration-150">
    <span x-text="selectedValue || 'Select an option'"
          :class="!selectedValue ? 'text-text-tertiary' : 'text-text-primary'"></span>
    <ChevronDownIcon size={13} class="text-text-tertiary" />
  </button>

  <!-- Dropdown menu -->
  <div x-show="dropdownOpen" @click.outside="dropdownOpen = false" x-cloak
       class="absolute z-50 mt-1 w-full
         bg-card border border-border rounded-md shadow-dropdown
         overflow-hidden max-h-48 overflow-y-auto">

    <template x-if="options.length === 0">
      <div class="px-3 py-2 text-xs text-text-tertiary">No options available</div>
    </template>

    <template x-for="option in options" :key="option">
      <div @click="selectOption(option)"
           class="px-3 py-2 text-sm text-text-primary
             hover:bg-surface cursor-pointer transition-colors duration-100"
           :class="option === selectedValue ? 'bg-brand-subtle text-brand-primary font-medium' : ''"
           x-text="option"></div>
    </template>

  </div>
</div>
```

Rules:
- Use `@click.outside` to close the dropdown when clicking outside.
- Use `x-cloak` on the dropdown menu to prevent flash of unstyled content.
- Selected item must use `bg-brand-subtle text-brand-primary font-medium`.
- Empty state must use `text-xs text-text-tertiary`.
- Max height `max-h-48` with `overflow-y-auto` for scrollable lists.

### 7.4 Modal

Backdrop must be `bg-[#1A2B3C]/30 backdrop-blur-sm`.

**Never combine `backdrop-filter` with `overflow-y-auto` on the same element.** Browsers fail to render backdrop blur correctly when the element is scrollable, resulting in unblurred white areas at the top or edges. Always separate the backdrop layer from the scrollable content container.

Modals must use `x-show` (not `x-if`) combined with `x-cloak` so that Alpine.js initializes all nested directives (especially `x-for` in dropdowns) at page load. This ensures reactive data bindings are wired before any async fetch completes.

```html
<div x-show="showModal" x-cloak class="fixed inset-0 z-50">
  <!-- Backdrop layer: fixed, no overflow, handles blur -->
  <div class="fixed inset-0 bg-[#1A2B3C]/30 backdrop-blur-sm"
       @click.self="closeModal"></div>

  <!-- Scrollable content layer: separate from backdrop -->
  <div class="fixed inset-0 overflow-y-auto">
    <div class="flex min-h-full items-center justify-center p-4">
      <div class="w-full max-w-md bg-card rounded-lg shadow-modal border border-border">

        <div class="flex items-center justify-between px-5 py-4 border-b border-border-subtle">
          <h2 class="text-sm font-semibold text-text-primary">Modal Title</h2>
          <button @click="closeModal" class="w-7 h-7 inline-flex items-center justify-center
            rounded text-text-tertiary hover:text-text-primary hover:bg-surface
            transition-colors duration-100">
            <XIcon size={14} />
          </button>
        </div>

        <div class="px-5 py-4 text-sm text-text-primary">
          Modal body.
        </div>

        <div class="flex items-center justify-end gap-2 px-5 py-4
          border-t border-border-subtle">
          <button class="...secondary button...">Cancel</button>
          <button class="...primary button...">Confirm</button>
        </div>

      </div>
    </div>
  </div>
</div>
```

Rules:
- **Always use `x-show` + `x-cloak`, never `x-if`, for modal overlays.**
- **Always separate the backdrop (`fixed inset-0` with `backdrop-blur-sm`) from the scrollable container (`fixed inset-0 overflow-y-auto`).**
- **Never put `overflow-y-auto` and `backdrop-blur-sm` on the same element.**
- Use `@click.self` on the backdrop to close when clicking outside the modal card.
- Allowed widths: `max-w-sm`, `max-w-md`, `max-w-lg`, `max-w-2xl`.

### 7.5 Badge / Status Label

```html
<span class="inline-flex items-center gap-1 px-2 py-0.5
  rounded-sm text-xs font-medium
  bg-status-success-bg text-status-success">
  <span class="w-1.5 h-1.5 rounded-full bg-status-success flex-shrink-0"></span>
  Running
</span>
```

Rules:

- Live status dots may use `animate-pulse`.
- Badge container must use `rounded-sm`, never `rounded-full`.

### 7.6 Table

```html
<div class="w-full overflow-x-auto">
  <table class="w-full text-sm">
    <thead>
      <tr class="border-b border-border">
        <th class="px-4 py-2.5 text-left text-xs font-medium
          text-text-secondary uppercase tracking-wide whitespace-nowrap">
          Column
        </th>
        <th class="px-4 py-2.5 text-left text-xs font-medium
          text-text-secondary uppercase tracking-wide font-mono">
          ID / URL
        </th>
        <th class="px-4 py-2.5 w-20"></th>
      </tr>
    </thead>
    <tbody class="divide-y divide-border-subtle">
      <tr class="hover:bg-surface transition-colors duration-100 group">
        <td class="px-4 py-2.5 text-sm text-text-primary">Value</td>
        <td class="px-4 py-2.5 text-[13px] font-mono text-text-secondary">abc-123</td>
        <td class="px-4 py-2.5">
          <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity justify-end">
            <button class="...icon button..."><EditIcon size={13} /></button>
            <button class="...icon button..."><TrashIcon size={13} /></button>
          </div>
        </td>
      </tr>
    </tbody>
  </table>
</div>
```

Table rules:

- Approximate row height: 40px.
- Header: `text-xs uppercase tracking-wide text-text-secondary`.
- Row actions are hidden by default and shown on `group-hover`.
- Technical columns use `font-mono text-[13px]`.
- Colored square row actions are forbidden.

### 7.7 Copyable Technical Field

Long URLs, hostnames, tokens, and IDs must never be shown as bare text.

```html
<div class="flex items-center gap-2 px-3 py-1.5
  bg-surface border border-border rounded
  font-mono text-[13px] text-text-primary">
  <span class="flex-1 truncate">container-id-abc123def456</span>
  <button class="text-text-tertiary hover:text-text-secondary flex-shrink-0 transition-colors">
    <CopyIcon size={13} />
  </button>
</div>
```

### 7.8 Card

```html
<div class="bg-card border border-border rounded-md p-4">
  <!-- content -->
</div>
```

Rules:

- No shadow.
- Use `rounded-md`.
- Nested cards are forbidden.
- Prefer Section + Divider instead of extra cards when possible.

### 7.9 Section + Divider

```html
<section class="py-4">
  <h3 class="text-xs font-semibold text-text-secondary uppercase tracking-wide mb-3">
    Section Title
  </h3>
  <!-- content -->
</section>
<div class="border-t border-border-subtle" />
<section class="py-4">
  <!-- next section -->
</section>
```

### 7.10 Standard Page Header

Full-width hero blocks are forbidden.

```html
<div class="flex items-start justify-between mb-6">
  <div>
    <h1 class="text-lg font-semibold text-text-primary">Page Title</h1>
    <p class="text-xs text-text-secondary mt-0.5">Short description</p>
    <div class="flex items-center gap-2 mt-1.5">
      <span class="...status badge...">Running</span>
      <span class="text-xs text-text-tertiary">Updated 2 minutes ago</span>
    </div>
  </div>
  <button class="...primary button...">+ New App</button>
</div>
```

Header rules:

- Only one CTA per page.
- No hero color block, no gradient, no full-width banner.
- Title must stay at `text-lg font-semibold`.

### 7.11 Sidebar

```html
<aside class="w-56 bg-sidebar border-r border-border flex flex-col h-screen">

  <div class="h-12 flex items-center px-4 border-b border-border">
    <span class="text-sm font-semibold text-text-primary tracking-tight">Pruvon</span>
  </div>

  <nav class="flex-1 px-2 py-3 space-y-0.5 overflow-y-auto">
    <a href="#" class="flex items-center gap-2.5 border-l-2 border-brand-accent
      px-3 py-2 text-brand-accent text-sm font-medium">
      <DashboardIcon size={15} />
      Dashboard
    </a>

    <a href="#" class="flex items-center gap-2.5 border-l-2 border-transparent
      px-3 py-2 text-text-secondary hover:text-text-primary
      text-sm transition-colors duration-100">
      <AppsIcon size={15} />
      Applications
    </a>
  </nav>

  <div class="px-4 py-3 border-t border-border">
    <span class="text-xs text-text-tertiary font-mono">v0.1.0</span>
  </div>

</aside>
```

Rules:

- Width: `w-56`.
- Nav icons: 15px.
- Active item: `border-l-2 border-brand-accent` + `text-brand-accent`, no background.
- Inactive item: `border-l-2 border-transparent` + `text-text-secondary hover:text-text-primary`.
- No icon tiles or colored icon containers.

### 7.12 Tabs

```html
<div class="flex border-b border-border mb-5">
  <button class="px-4 py-2.5 text-sm font-medium
    text-brand-primary border-b-2 border-brand-primary -mb-px">
    Overview
  </button>

  <button class="px-4 py-2.5 text-sm font-medium
    text-text-secondary hover:text-text-primary
    border-b-2 border-transparent -mb-px
    transition-colors duration-100">
    Logs
  </button>
</div>
```

### 7.13 Tooltip

```html
<div class="absolute z-50 px-2 py-1
  bg-[#1A2B3C] text-white text-xs rounded-sm
  shadow-tooltip whitespace-nowrap pointer-events-none">
  Tooltip text
</div>
```

### 7.14 Danger Zone

Permanent destructive actions must always live in the last section of the page.

```html
<section class="py-4 mt-4">
  <div class="border border-status-danger/30 rounded-md p-4 bg-status-danger-bg/30">
    <h3 class="text-sm font-semibold text-status-danger mb-1">Danger Zone</h3>
    <p class="text-xs text-text-secondary mb-3">
      These actions cannot be undone.
    </p>
    <button class="...destructive button...">Delete App</button>
  </div>
</section>
```

## 8. Layout

### App Shell

```text
┌────────────────────────────────────────────┐
│  Sidebar 224px  │  Main Content            │
│  bg-sidebar     │  bg-surface              │
│  border-r       │  px-6 py-5               │
└────────────────────────────────────────────┘
```

### Content Width

| Page Type | Max Width |
|---|---|
| Form, detail page | `max-w-4xl` |
| List, table | `max-w-6xl` |
| Logs, full-width tables | no width limit |

## 9. Icon System

Use Lucide icons only.

| Context | Size |
|---|---|
| Sidebar nav | `size={15}` |
| Button icon | `size={14}` |
| Table / inline | `size={13}` |
| Page title | `size={18}` |
| Status indicator | Prefer colored dot |

Never place icons inside colored square or circle tiles.

## 10. Motion

```css
transition-colors duration-150
transition-opacity duration-100
```

Rules:

- `duration-150` for button hover, links, border colors.
- `duration-100` for row action opacity and dropdown item hover.
- No complex animation.
- `animate-bounce` and `animate-spin` are forbidden except actual loading spinners.

## 10.1 Loading States (Async Actions)

Any action triggered by a user click that results in an async API call must show a processing state to prevent duplicate submissions and provide clear feedback.

**Rules:**
- Use an Alpine.js boolean state (e.g., `isSaving`, `isProcessing`) scoped to the action.
- Set the flag to `true` before the `fetch` call and reset to `false` in a `finally` block.
- The primary action button must display a spinning loader icon and change its label to "Processing..." while active.
- Disable the primary action button while processing.
- Disable the cancel/close button (including the modal X button) while processing.
- Apply `disabled:cursor-not-allowed disabled:opacity-50` to all disabled buttons during processing.

**Spinner markup:**

```html
<svg class="h-3.5 w-3.5 animate-spin" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
</svg>
```

**Button pattern:**

```html
<button
  @click="isProcessing = true; fetch(...).finally(() => isProcessing = false)"
  :disabled="isProcessing"
  class="inline-flex h-8 items-center gap-1.5 rounded border border-brand-primary bg-brand-primary px-3 text-sm font-medium text-white transition-colors duration-150 hover:bg-brand-hover disabled:cursor-not-allowed disabled:opacity-50">
  <svg x-show="isProcessing" class="h-3.5 w-3.5 animate-spin" ...>...</svg>
  <span x-text="isProcessing ? 'Processing...' : 'Save'"></span>
</button>
```

## 10.2 Alpine.js Gotchas

The following patterns are known to cause issues in Alpine.js. Never use them.

**Rule 1: Never make `@click` handlers `async`.**

Alpine's event handling system does not properly handle promises returned from event handlers. An `async` `@click` handler will silently fail (the handler runs but the reactive context is lost and DOM mutations may not be applied). All `@click`, `@change`, `@submit`, and other event handler methods **must** be synchronous.

If a handler needs to perform async work (e.g., `fetch`), keep the handler synchronous and use the fire-and-forget pattern — call the async method without `await`. The parent handler finishes immediately, and the async method writes to reactive state when it completes.

```javascript
// CORRECT: synchronous handler
saveUser() {
    this.isSavingUser = true;
    this.doAsyncSave();  // fire-and-forget, no await
},

async doAsyncSave() {
    try {
        const response = await fetch('/api/...', { method: 'PUT', ... });
        // ... reactive state updates still work
    } finally {
        this.isSavingUser = false;
    }
}

// WRONG: async handler — will silently fail
async saveUser() {
    this.isSavingUser = true;
    const response = await fetch('/api/...', { method: 'PUT', ... });
    this.isSavingUser = false;  // This may never execute
}
```

**Rule 2: Avoid `<template x-for>` inside `<select>` elements.**

Browser form-filling extensions (e.g., LastPass) and some browser rendering engines do not reliably parse `<template>` tags when they appear inside `<select>`. This causes the generated `<option>` elements to not render until the browser performs a forced reflow (e.g., opening DevTools).

Instead, use `x-html` with a template literal to populate `<option>` elements. `x-html` replaces the select's `innerHTML`, which forces a full DOM re-parse and ensures options are always rendered:

```html
<select x-model="selectedValue" x-html="`<option value=''>Select...</option>${items.map(item => `<option value='${item}'>${item}</option>`).join('')}`"></select>
```

For selects with conditional data (e.g., service names depend on selected type), use a ternary expression:

```html
<select x-model="selectedService"
  x-html="`<option value=''>Select...</option>${(condition ? list : []).map(item => `<option value='${item}'>${item}</option>`).join('')}`">
</select>
```

Add an HTML comment above the `<select>` explaining the workaround so future maintainers understand why `x-for` is not used.

## 11. Screen-Specific Decisions

### Dashboard

- Keep the page header short.
- Use minimal overview metric cards.
- CPU, RAM, and Disk should use restrained donut or linear progress, not generic flashy circles.
- Prefer Section + Divider over too many cards.

### Applications List

- Dense table layout using `max-w-6xl`.
- Row-end actions must be icon buttons shown on hover.
- Only one create button in the page header.

### Application Detail

- Use light local navigation similar to the sidebar.
- Sections: Summary, Processes, Env Vars, Services, Danger Zone.
- Stop / Restart / Rebuild should use secondary or tertiary hierarchy, not destructive styling.
- Donut metrics should use thinner strokes and larger numbers.

### Services List

- Empty states must preserve composition.
- No floating action button.
- Keep the table compact.

### Service Detail

- Technical values must use the copyable field component.
- Keep status strip restrained.
- Danger Zone must be the last section.

### Settings

- Form language should feel refined and spacious.
- Helper text should stay secondary or collapsible, not large content blocks.
- Tabs should use the `border-b-2` premium style.

### Logs

- Filters should stay tight and functional.
- Dense but readable table using `text-xs` or `text-sm`.
- Action labels should use tonal neutral badges, not colorful candy tags.
- Technical JSON should use `font-mono text-[13px]` and be collapsible.

### Backups

- Filters, categories, and list should feel like one toolbar.
- Download should be tertiary or icon button.
- The table remains the primary output.

### Plugins

- Make Installed vs Available visually distinct through weight, not loud color.
- Raw GitHub URLs must use the copyable field component.
- Install action should be a row-end secondary button, not a button strip.

### Login

- Centered card layout with brand header above the form.
- Logo image (`/static/images/logo/logo.webp`) centered at top, `h-10`.
- App name uses `font-serif text-2xl text-text-primary` below the logo.
- Subtitle uses `text-sm text-text-secondary` below the app name.
- Brand header sits outside the card, separated by `mb-8`.
- The card itself contains the form header ("Sign in"), fields, and actions.
- Container uses `flex-col justify-center` to vertically center on the page.
- Max width of the entire column is `max-w-[420px]`.

## 12. Hard Bans

The following are permanently forbidden in this design system:

| # | Forbidden | Why |
|---|---|---|
| 1 | Full-width saturated hero block | Makes admin UI look like a landing page |
| 2 | Gradients (`bg-gradient-*`) | Feels generic and artificial |
| 3 | Heavy shadow (`shadow-lg`, `shadow-xl`) | Too heavy, not premium |
| 4 | Blue or purple brand color | Cliche, low character |
| 5 | Gray buttons (`bg-gray-*`) | Breaks the color system |
| 6 | Colored icon tile | Feels AI-generated and noisy |
| 7 | `rounded-full` badge | Violates the system |
| 8 | FAB and top CTA on the same page | Breaks action hierarchy |
| 9 | Border radius above 8px except modal | Over-rounded |
| 10 | Colored square row-end actions | Hurts table readability |
| 11 | Bare long URL / container ID | Ugly and hard to scan |
| 12 | Nested card | Visual noise |
| 13 | Blue link (`text-blue-*`) | Use `text-link` instead |
| 14 | Opaque modal backdrop | Must use `bg-[#1A2B3C]/30 backdrop-blur-sm` |
| 15 | More than 3 font weights | Only 300, 400, 500 are allowed |
| 16 | Inter font | Use DM Sans |
| 17 | `system-ui` or `sans-serif` alone | Explicit font family is required |
| 18 | Floating action button | Page header CTA is enough |

## 13. Agent Checklist

Before finalizing any UI change, confirm all of the following:

- [ ] No color outside the palette was introduced.
- [ ] DM Sans / DM Mono / Instrument Serif are used correctly.
- [ ] No banned anti-pattern appears.
- [ ] The page header follows the standard template.
- [ ] There is no second primary CTA on the page.
- [ ] Technical values use the copyable field pattern.
- [ ] Modal backdrop is `bg-[#1A2B3C]/30 backdrop-blur-sm`.
- [ ] Dropdown background is `bg-card`.
- [ ] A card was not used where Section + Divider would be better.
- [ ] Table row actions are icon buttons, not colored squares.
- [ ] Danger Zone is at the bottom in its own section.
- [ ] Async actions (save, delete, confirm) show a processing spinner and disable buttons during the operation.

Last updated: Pruvon UI v2 Design System

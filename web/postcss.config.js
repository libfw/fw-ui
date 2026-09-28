// ⛔ `@tailwindcss/postcss`, not `tailwindcss`. Tailwind v4 moved its PostCSS
// plugin into a separate package and refuses the old spelling outright:
//
//   [vite:css] [postcss] It looks like you're trying to use `tailwindcss`
//   directly as a PostCSS plugin. The PostCSS plugin has moved to a separate
//   package…
//
// which is the whole of what `frontend build` said on the v4 bump.
import tailwindcss from '@tailwindcss/postcss'
import autoprefixer from 'autoprefixer'
export default { plugins: [tailwindcss, autoprefixer] }

import daisyui from 'daisyui'

export default {
  content: ['./index.html', './src/**/*.{svelte,js}'],
  theme: { extend: {} },
  plugins: [daisyui],
  daisyui: {
    themes: ['dark', 'light', 'night', 'dracula', 'synthwave', 'cyberpunk', 'business', 'emerald'],
    logs: false,
  },
}

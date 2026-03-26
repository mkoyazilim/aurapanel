/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        brand: {
          50: '#ecfdf5',
          100: '#d1fae5',
          200: '#a7f3d0',
          300: '#6ee7b7',
          400: '#34d399',
          500: '#10b981', // Emerald primary
          600: '#059669',
          700: '#047857',
          800: '#065f46',
          900: '#064e3b',
        },
        panel: {
          dark: '#0f172a',    // Slate 900
          darker: '#020617',  // Slate 950
          card: '#1e293b',    // Slate 800
          border: '#334155'   // Slate 700
        }
      }
    },
  },
  plugins: [],
}

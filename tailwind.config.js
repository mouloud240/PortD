/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./views/**/*.{go,html,templ}",
    "./internal/**/*.{go,html}",
  ],
  theme: {
    extend: {
      colors: {
        brand: {
          blue: "#003da5",
          "blue-hover": "#002e80",
          navy: "#172b4d",
          green: "#087f44",
          "green-soft": "#e7f6ed",
          amber: "#9a5500",
          "amber-soft": "#fff5dd",
          red: "#bd2d2d",
          "red-soft": "#fff0f0",
          ink: "#182536",
          muted: "#536274",
          line: "#dbe3ed",
          ground: "#f5f8fb",
        },
      },
      fontFamily: {
        sans: [
          "Inter",
          "ui-sans-serif",
          "system-ui",
          "-apple-system",
          "BlinkMacSystemFont",
          '"Segoe UI"',
          "sans-serif",
        ],
        mono: [
          "ui-monospace",
          "SFMono-Regular",
          "Menlo",
          "Consolas",
          "monospace",
        ],
      },
    },
  },
  plugins: [],
};

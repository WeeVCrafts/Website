/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./web/**/*.templ', './internal/web/**/*.go', './internal/transport/web/templates/**/*.html'],
  theme: {
    extend: {
      colors: {
        paper: '#fffaf4',
        mist: '#eef2f6',
        ink: '#162238',
        indigo: '#1e3158',
        citrus: '#f79032',
        line: '#cdd3dc',
      },
      fontFamily: {
        sans: ['"DM Sans"', 'Arial', 'sans-serif'],
        display: ['"Playfair Display"', 'Georgia', 'serif'],
        serif: ['"Playfair Display"', 'Georgia', 'serif'],
      },
    },
  },
  plugins: [],
};

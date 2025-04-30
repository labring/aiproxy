import { AppRouter } from "@/routes"
import { ThemeProvider } from "./handler/ThemeProvider"

function App() {
  return (
    <ThemeProvider defaultTheme="system" storageKey="aiproxy-theme">
      <AppRouter />
    </ThemeProvider>
  )
}

export default App
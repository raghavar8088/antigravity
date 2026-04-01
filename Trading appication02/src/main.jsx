import React from "react";
import { createRoot } from "react-dom/client";
import App from "./App.jsx";

class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props);
    this.state = { error: null };
  }
  static getDerivedStateFromError(error) {
    return { error };
  }
  render() {
    if (this.state.error) {
      return (
        <div style={{ padding: 40, fontFamily: "monospace", background: "#fff0f0", minHeight: "100vh" }}>
          <h2 style={{ color: "red" }}>React Render Error</h2>
          <pre style={{ color: "#900", whiteSpace: "pre-wrap" }}>{this.state.error.toString()}</pre>
          <pre style={{ color: "#555", fontSize: 12, whiteSpace: "pre-wrap" }}>{this.state.error.stack}</pre>
        </div>
      );
    }
    return this.props.children;
  }
}

createRoot(document.getElementById("root")).render(
  <ErrorBoundary>
    <App />
  </ErrorBoundary>
);

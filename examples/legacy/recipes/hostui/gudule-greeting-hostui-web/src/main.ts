import "./style.css";

import { renderApp } from "./ui";

const root = document.querySelector<HTMLElement>("#app");

if (!root) {
  throw new Error("Missing #app root");
}

renderApp(root);

import type { GreetingLanguage } from "./grpc-client";
import { listLanguages, sayHello } from "./grpc-client";

export function renderApp(root: HTMLElement): void {
  root.innerHTML = `
    <main class="shell">
      <section class="hero">
        <h1>Gudule Greeting<br />Web</h1>
        <p>
          A browser client for the shared GreetingService contract. The page keeps
          the UI thin, defaults to the current origin, and lets any compatible
          greeting daemon own the language catalog.
        </p>
      </section>
      <section class="panel greeting-card">
        <div class="controls">
          <label class="field">
            <span>Language</span>
            <select data-role="language-select"></select>
          </label>
          <label class="field">
            <span>Your name</span>
            <input data-role="name-input" type="text" value="World" placeholder="World" />
          </label>
          <button data-role="greet-button" type="button">Greet</button>
          <div class="status" data-role="status"></div>
        </div>
        <div class="greeting-output" data-role="greeting-output">
          <p>Loading languages…</p>
        </div>
      </section>
    </main>
  `;

  const languageSelect = root.querySelector<HTMLSelectElement>('[data-role="language-select"]');
  const greetingOutput = root.querySelector<HTMLElement>('[data-role="greeting-output"]');
  const nameInput = root.querySelector<HTMLInputElement>('[data-role="name-input"]');
  const greetButton = root.querySelector<HTMLButtonElement>('[data-role="greet-button"]');
  const status = root.querySelector<HTMLElement>('[data-role="status"]');

  if (!languageSelect || !greetingOutput || !nameInput || !greetButton || !status) {
    throw new Error("UI mount failed");
  }

  let languages: GreetingLanguage[] = [];
  let selectedLanguage: GreetingLanguage | undefined;

  const setStatus = (message: string, error = false) => {
    status.textContent = message;
    status.classList.toggle("error", error);
  };

  const setGreeting = (message: string, details = "") => {
    greetingOutput.innerHTML = details
      ? `<div><strong>${message}</strong><p>${details}</p></div>`
      : `<div><strong>${message}</strong></div>`;
  };

  const renderLanguages = () => {
    languageSelect.replaceChildren();
    for (const language of languages) {
      const option = document.createElement("option");
      option.value = language.code;
      option.textContent = `${language.native} (${language.name})`;
      languageSelect.appendChild(option);
    }
    if (selectedLanguage) {
      languageSelect.value = selectedLanguage.code;
    }
  };

  const greet = async () => {
    if (!selectedLanguage) {
      setGreeting("Select a language first", "The daemon already knows the catalog.");
      return;
    }
    const name = nameInput.value.trim() || "World";
    setStatus("Requesting greeting…");
    try {
      const greeting = await sayHello(name, selectedLanguage.code);
      setGreeting(greeting, `${selectedLanguage.native} · ${selectedLanguage.code}`);
      setStatus("");
    } catch (error) {
      setStatus(String(error), true);
      setGreeting("The daemon is not reachable", "Start a greeting daemon that serves the shared web client on this origin or set __GUDULE_DAEMON__.");
    }
  };

  languageSelect.addEventListener("change", () => {
    selectedLanguage = languages.find((language) => language.code === languageSelect.value);
  });
  greetButton.addEventListener("click", () => void greet());

  void (async () => {
    setStatus("Loading the language catalog…");
    try {
      languages = await listLanguages();
      selectedLanguage = languages.find((language) => language.code === "en") ?? languages[0];
      renderLanguages();
      setGreeting("Choose a language", "Select a language and press Greet.");
      setStatus("");
    } catch (error) {
      setStatus(String(error), true);
      setGreeting("The daemon is not reachable", "Run a greeting daemon with gRPC-Web support on this origin or set __GUDULE_DAEMON__.");
    }
  })();
}

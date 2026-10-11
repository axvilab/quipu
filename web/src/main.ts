import { mount } from "svelte";
import "./app.css";
import "./lib/theme.svelte"; // follow the system theme from the start
import App from "./App.svelte";

mount(App, { target: document.getElementById("app")! });

// Everything imported from ./mygo is generated from the Go code by
// `mygo generate`: calls and payloads are fully typed.
import { Todos, events, type Filter, type Todo } from "./mygo";

const $ = <T extends HTMLElement = HTMLElement>(selector: string) => document.querySelector<T>(selector)!;

let filter: Filter = "all";

async function render() {
  const [todos, stats] = await Promise.all([Todos.list(filter), Todos.stats()]);
  $("#list").replaceChildren(...todos.map(renderItem));
  $("#count").textContent = `${stats.active} of ${stats.total} left`;
}

function renderItem(todo: Todo) {
  const li = $<HTMLTemplateElement>("#item").content.firstElementChild!.cloneNode(true) as HTMLLIElement;
  li.classList.toggle("done", todo.done);
  const box = li.querySelector("input")!;
  box.checked = todo.done;
  box.addEventListener("change", () => run(Todos.toggle(todo.id)));
  li.querySelector("span")!.textContent = todo.title;
  li.querySelector(".remove")!.addEventListener("click", () => run(Todos.remove(todo.id)));
  return li;
}

// run reports errors returned by Go methods.
async function run(p: Promise<unknown>) {
  try {
    await p;
    $("#error").textContent = "";
  } catch (err) {
    $("#error").textContent = err instanceof Error ? err.message : String(err);
  }
}

$("#add").addEventListener("submit", (e) => {
  e.preventDefault();
  const input = $<HTMLInputElement>("#title");
  run(Todos.add(input.value).then(() => (input.value = "")));
});

$("#filters").addEventListener("click", (e) => {
  const button = (e.target as HTMLElement).closest<HTMLButtonElement>("button[data-filter]");
  if (!button) return;
  filter = button.dataset.filter as Filter;
  document.querySelectorAll("#filters button").forEach((b) => b.classList.toggle("active", b === button));
  render();
});

$("#clear").addEventListener("click", () => run(Todos.clearDone()));
$("#export").addEventListener("click", () => run(Todos.export()));

// Changes made in any window (or from the menu) are broadcast by Go.
events.todosChanged.on(() => render());

render();

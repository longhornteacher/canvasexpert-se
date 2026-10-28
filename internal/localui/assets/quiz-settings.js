(() => {
  const update = select => {
    const input = select.closest("td").querySelector("[data-quiz-minutes]");
    const custom = select.value === "minutes";
    input.hidden = !custom;
    input.required = custom;
    input.disabled = !custom;
  };
  document.querySelectorAll("[data-quiz-time]").forEach(select => {
    update(select);
    select.addEventListener("change", () => update(select));
  });
})();

// Renders the shared app shell (sidebar nav + topbar). Static markup only —
// no buyer- or seller-supplied content passes through innerHTML here.
function renderShell(active) {
  const links = [
    ["dashboard.html", "Dashboard"],
    ["inbox.html", "Inbox"],
    ["orders.html", "Orders"],
    ["gigs.html", "Gigs"],
    ["customers.html", "Customers"],
    ["ai-assistant.html", "AI Assistant"],
    ["analytics.html", "Analytics"],
    ["notifications.html", "Notifications"],
    ["settings.html", "Settings"],
  ];

  const nav = document.createElement("nav");
  nav.className = "sidebar";
  nav.innerHTML =
    '<div class="brand">Fiverr Seller Hub</div>' +
    '<ul class="nav-links">' +
    links
      .map(
        ([href, label]) =>
          `<li><a href="${href}" class="${active === href ? "active" : ""}">${label}</a></li>`
      )
      .join("") +
    '</ul>' +
    '<button id="logout-btn" class="nav-logout">Log out</button>';

  document.body.prepend(nav);

  document.getElementById("logout-btn").addEventListener("click", async () => {
    try {
      await apiFetch("/auth/logout", { method: "POST" });
    } catch (e) {
      /* ignore */
    }
    setAccessToken("");
    location.href = "/login.html";
  });
}

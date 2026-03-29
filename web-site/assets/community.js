(function () {
  var form = document.getElementById("joinForm");
  if (!form) {
    return;
  }

  var message = document.getElementById("formMessage");
  var submitButton = form.querySelector("button[type='submit']");
  var endpoint = form.getAttribute("data-endpoint") || "./api/community_join.php";

  function setMessage(text, ok) {
    if (!message) {
      return;
    }
    message.textContent = text;
    message.style.color = ok ? "#0d8a58" : "#be3a17";
  }

  function setSubmitting(active) {
    if (!submitButton) {
      return;
    }
    submitButton.disabled = active;
    submitButton.textContent = active ? "Sending..." : "Send Join Request";
    submitButton.style.opacity = active ? "0.8" : "1";
  }

  function isEmail(value) {
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
  }

  function readValue(id) {
    var node = document.getElementById(id);
    return node ? node.value.trim() : "";
  }

  async function submitRequest(payload) {
    var response = await fetch(endpoint, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Accept": "application/json"
      },
      body: JSON.stringify(payload)
    });

    var data = null;
    try {
      data = await response.json();
    } catch (_err) {
      data = { message: "Unexpected response." };
    }

    if (!response.ok || !data || data.status !== "success") {
      var reason = data && data.message ? data.message : "Request could not be submitted right now.";
      throw new Error(reason);
    }
  }

  form.addEventListener("submit", async function (event) {
    event.preventDefault();
    setMessage("", false);

    var payload = {
      full_name: readValue("fullName"),
      email: readValue("email"),
      company: readValue("company"),
      role: readValue("role"),
      focus: readValue("focus"),
      website_url: readValue("websiteUrl")
    };

    if (!payload.full_name || payload.full_name.length < 2) {
      setMessage("Please enter a valid full name.", false);
      return;
    }
    if (!isEmail(payload.email)) {
      setMessage("Please enter a valid email address.", false);
      return;
    }
    if (!payload.role) {
      setMessage("Please select your role.", false);
      return;
    }
    if (!payload.focus || payload.focus.length < 8) {
      setMessage("Please share a short focus area so we can route your request.", false);
      return;
    }

    setSubmitting(true);
    try {
      await submitRequest(payload);
      form.reset();
      setMessage("Request submitted successfully. We will contact you through your work email.", true);
    } catch (error) {
      setMessage(error && error.message ? error.message : "Request could not be submitted right now.", false);
    } finally {
      setSubmitting(false);
    }
  });
})();

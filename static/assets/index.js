document.addEventListener("DOMContentLoaded", () => {
  const form = document.getElementById("loginForm");
  const toggleButton = document.getElementById("toggleButton");
  const formTitle = document.getElementById("formTitle");
  const usernameLabel = document.getElementById("username");
  const fetchAccountButton = document.getElementById("fetchAccountButton");

  function switchToSignUp() {
    formTitle.textContent = "Sign Up";
    toggleButton.textContent = "Login";
    document.getElementById("password").required = true;
    usernameLabel.style.display = "block";
  }

  function switchToLogin() {
    formTitle.textContent = "Login";
    toggleButton.textContent = "Sign Up";
    document.getElementById("password").required = true;
    usernameLabel.style.display = "block";
  }

  toggleButton.addEventListener("click", (event) => {
    event.preventDefault();
    if (formTitle.textContent === "Login") {
      switchToSignUp();
    } else {
      switchToLogin();
    }
  });

  form.addEventListener("submit", (event) => {
    event.preventDefault();

    const email = document.getElementById("email").value;
    const username = document.getElementById("username").value;
    const password = document.getElementById("password").value;

    const formData = new FormData();
    formData.append("email", email);
    formData.append("username", username);
    formData.append("password", password);

    const endpoint =
      formTitle.textContent === "Login" ? "/api/login" : "/api/signup";

    fetch(endpoint, {
      method: "POST",
      body: formData,
      redirect: "follow",
    })
      .then((response) => {
        if (response.redirected) {
          window.location.href = response.url;
          return Promise.reject("Redirecting...");
        }

        if (!response.ok) {
          return response.text().then((text) => {
            throw new Error(text || `HTTP error! Status: ${response.status}`);
          });
        }
        if (
          response.headers.get("content-type")?.includes("application/json")
        ) {
          return response.json();
        }
        return response.text();
      })
      .then((data) => {
        if (typeof data === "object") {
          alert(
            `Success: ${data.message || "Operation completed successfully"}`,
          );
        } else {
          alert(data);
        }
      })
      .catch((error) => {
        if (error.message !== "Redirecting...") {
          console.error("Error:", error);
          alert(`Error: ${error.message || "Something went wrong"}`);
        }
      });
  });

  fetchAccountButton.addEventListener("click", () => {
    fetch("/api/account", {
      method: "GET",
      credentials: "include",
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error("Network response was not ok");
        }
        return response.json();
      })
      .then((data) => {
        console.log("Account Details:", data);
        alert(`Account Details: ${JSON.stringify(data)}`);
      })
      .catch((error) => {
        console.error("Error fetching account details:", error);
        alert("Failed to fetch account details.");
      });
  });
});

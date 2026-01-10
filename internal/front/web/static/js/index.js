function displayErrorMsgDiv(errDivId, msg) {
    const errDiv = document.getElementById(errDivId);
    if (errDiv) {
        errDiv.textContent = msg;
        errDiv.classList.add("show");
    }

    setTimeout(() => {
        errDiv.classList.remove("show");
    }, 2000);
}

document.addEventListener('htmx:afterRequest', function (event) {
    const trigElement = event.detail.elt;
    const xhr = event.detail.xhr;
    console.log(trigElement);

    if (trigElement.id === 'login-form') {
        if (xhr.status >= 200 && xhr.status < 300) {
            window.location.href = "/api/v1/auth/dashboard"
        } else if (xhr.status >= 400) {
            displayErrorMsgDiv('login-error', xhr.responseText);
            window.location.href = "/api/v1/login"
        }
    } else if (trigElement.id === 'register-form') {
        console.log(xhr.responseText);
        if (xhr.status >= 200 && xhr.status < 300) {
            window.location.href = "/api/v1/login"
        } else if (xhr.status >= 400) {
            displayErrorMsgDiv('register-error', xhr.responseText);
        }
    }
});



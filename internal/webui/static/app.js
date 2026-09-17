const state = {
  user: null,
  products: [],
  operations: [], // {id, label, status, errorMessage} yang sedang/telah dilacak
  pagination: { limit: 20, offset: 0, count: 0 },
};

// ---------- Util ----------

// Endpoint yang secara wajar bisa mengembalikan 401 tanpa berarti "sesi habis"
// (mis. saat login gagal, atau saat cek sesi pertama kali sebelum login).
const AUTH_PROBE_PATHS = new Set(["/api/auth/login", "/api/auth/me"]);

async function api(path, options = {}) {
  let res;
  try {
    res = await fetch(path, {
      ...options,
      credentials: "include",
      cache: "no-store", // cegah browser menyajikan response GET lama saat polling
      headers: {
        "Content-Type": "application/json",
        ...(options.headers || {}),
      },
    });
  } catch (err) {
    // Server tidak terjangkau / koneksi terputus.
    return null;
  }

  if (res.status === 401 && !AUTH_PROBE_PATHS.has(path)) {
    handleSessionExpired();
  }

  return res;
}

function showNetworkError() {
  alert("Tidak dapat terhubung ke server. Periksa koneksi internet Anda dan coba lagi.");
}

// Dipanggil saat API menolak permintaan karena sesi sudah tidak valid lagi
// (mis. cookie kedaluwarsa atau logout dari tab lain).
function handleSessionExpired() {
  if (state.user === null) return; // sudah di layar login, tidak perlu diulang
  state.user = null;
  state.operations = [];
  showLogin();
  const errorEl = document.getElementById("login-error");
  if (errorEl) {
    errorEl.textContent = "Sesi Anda telah berakhir, silakan login kembali";
    show(errorEl);
  }
}

function show(el) { el.classList.remove("hidden"); }
function hide(el) { el.classList.add("hidden"); }

function escapeHTML(str) {
  const div = document.createElement("div");
  div.textContent = str ?? "";
  return div.innerHTML;
}

// ---------- Auth ----------

async function checkSession() {
  const res = await api("/api/auth/me");
  if (!res) {
    // Gagal terhubung saat load awal; tetap tampilkan layar login agar user bisa coba lagi.
    showLogin();
    return;
  }
  if (res.status === 200) {
    const body = await res.json();
    state.user = body.user;
    showApp();
  } else {
    showLogin();
  }
}

function showLogin() {
  hide(document.getElementById("view-app"));
  show(document.getElementById("view-login"));
}

function showApp() {
  hide(document.getElementById("view-login"));
  show(document.getElementById("view-app"));
  document.getElementById("current-user-name").textContent = state.user.name;
  document.getElementById("current-user-role").textContent = state.user.role;
  loadProducts();
}

document.getElementById("login-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const errorEl = document.getElementById("login-error");
  hide(errorEl);

  const email = document.getElementById("login-email").value.trim();
  const password = document.getElementById("login-password").value;

  const res = await api("/api/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });

  if (!res) {
    errorEl.textContent = "Tidak dapat terhubung ke server, coba lagi";
    show(errorEl);
    return;
  }

  if (res.status === 200) {
    const body = await res.json();
    state.user = body.user;
    document.getElementById("login-form").reset();
    showApp();
  } else {
    const body = await res.json().catch(() => ({}));
    errorEl.textContent = body.error || "Email atau password salah";
    show(errorEl);
  }
});

document.getElementById("logout-btn").addEventListener("click", async () => {
  await api("/api/auth/logout", { method: "POST" });
  state.user = null;
  showLogin();
});

// ---------- Tabs ----------

document.querySelectorAll(".tab").forEach((btn) => {
  btn.addEventListener("click", () => {
    document.querySelectorAll(".tab").forEach((b) => b.classList.remove("active"));
    btn.classList.add("active");

    document.querySelectorAll(".tab-panel").forEach((p) => hide(p));
    show(document.getElementById(`tab-${btn.dataset.tab}`));
  });
});

// ---------- Produk ----------

async function loadProducts(offset = state.pagination.offset) {
  console.log(`[debug] loadProducts dipanggil, offset=${offset}`);
  const params = new URLSearchParams({
    limit: String(state.pagination.limit),
    offset: String(offset),
  });

  const res = await api(`/api/products?${params.toString()}`);
  if (!res) {
    console.log("[debug] loadProducts: fetch gagal total (res null)");
    showNetworkError();
    return;
  }
  console.log(`[debug] loadProducts: respons status ${res.status}`);
  if (res.status === 401) return; // sudah dialihkan ke login oleh handleSessionExpired
  if (res.status !== 200) return;

  const body = await res.json();
  const products = body.data || [];
  console.log(`[debug] loadProducts: menerima ${products.length} produk`, products);

  // Halaman ini kosong (mis. produk terakhir di halaman baru saja dihapus):
  // mundur satu halaman alih-alih menampilkan tabel kosong padahal masih ada data.
  if (products.length === 0 && offset > 0) {
    return loadProducts(Math.max(0, offset - state.pagination.limit));
  }

  state.products = products;
  state.pagination.offset = body.pagination?.offset ?? offset;
  state.pagination.limit = body.pagination?.limit ?? state.pagination.limit;
  state.pagination.count = body.pagination?.count ?? products.length;

  console.log("[debug] loadProducts: memanggil renderProductTable()");
  renderProductTable();
  renderPaginationControls();
}

function renderProductTable() {
  console.log(`[debug] renderProductTable: menampilkan ${state.products.length} baris`);
  const body = document.getElementById("product-table-body");
  const empty = document.getElementById("product-empty");
  const head = document.getElementById("product-table-head");

  body.innerHTML = "";

  head.innerHTML =
    "<th>SKU</th><th>Nama</th><th>Kategori</th><th>Brand</th><th>Harga</th><th></th>";

  if (state.products.length === 0) {
    show(empty);
    return;
  }
  hide(empty);

  for (const p of state.products) {
    const tr = document.createElement("tr");
    tr.innerHTML =
      `<td>${escapeHTML(p.sku)}</td>` +
      `<td>${escapeHTML(p.name)}</td>` +
      `<td>${escapeHTML(p.category)}</td>` +
      `<td>${escapeHTML(p.brand)}</td>` +
      `<td>${escapeHTML(p.price)}</td>` +
      `<td class="actions">
         <button data-action="edit" data-id="${p.id}">Ubah</button>
         <button data-action="delete" data-id="${p.id}" class="btn-secondary">Hapus</button>
       </td>`;
    body.appendChild(tr);
  }
}

// Membuat kontrol pagination sekali saja dan menaruhnya tepat setelah tabel produk,
// tanpa bergantung pada markup HTML yang sudah ada.
function ensurePaginationControls() {
  let el = document.getElementById("product-pagination");
  if (el) return el;

  el = document.createElement("div");
  el.id = "product-pagination";
  el.className = "pagination";
  el.innerHTML =
    `<button id="product-prev-btn" type="button" class="btn-secondary">&laquo; Sebelumnya</button>` +
    `<span id="product-page-info" class="muted"></span>` +
    `<button id="product-next-btn" type="button" class="btn-secondary">Berikutnya &raquo;</button>`;

  const table = document.getElementById("product-table-body")?.closest("table");
  const anchor = table || document.getElementById("product-empty");
  if (anchor && anchor.parentElement) {
    anchor.parentElement.insertBefore(el, anchor.nextSibling);
  }

  document.getElementById("product-prev-btn").addEventListener("click", () => {
    loadProducts(Math.max(0, state.pagination.offset - state.pagination.limit));
  });
  document.getElementById("product-next-btn").addEventListener("click", () => {
    loadProducts(state.pagination.offset + state.pagination.limit);
  });

  return el;
}

function renderPaginationControls() {
  ensurePaginationControls();

  const { limit, offset, count } = state.pagination;
  const prevBtn = document.getElementById("product-prev-btn");
  const nextBtn = document.getElementById("product-next-btn");
  const info = document.getElementById("product-page-info");
  if (!prevBtn || !nextBtn || !info) return;

  prevBtn.disabled = offset <= 0;
  // Backend tidak mengirim total baris; anggap masih ada halaman berikutnya
  // selama jumlah baris yang diterima sama dengan limit yang diminta.
  nextBtn.disabled = count < limit;

  const start = count === 0 ? 0 : offset + 1;
  const end = offset + count;
  info.textContent = count === 0 ? "Tidak ada data" : `Menampilkan ${start}–${end}`;
}

document.getElementById("product-table-body").addEventListener("click", async (e) => {
  const btn = e.target.closest("button[data-action]");
  if (!btn) return;

  const id = btn.dataset.id;
  const product = state.products.find((p) => String(p.id) === id);
  if (!product) return;

  if (btn.dataset.action === "edit") {
    openProductForm(product);
  }

  if (btn.dataset.action === "delete") {
    if (!confirm(`Hapus produk "${product.name}"?`)) return;

    // Delete membutuhkan version produk saat ini (optimistic concurrency).
    const res = await api(
      `/api/products/${id}?version=${product.version}`,
      { method: "DELETE" }
    );

    if (!res) {
      showNetworkError();
      return;
    }
    if (res.status === 401) return; // sudah dialihkan ke login

    if (res.status >= 400) {
      const errBody = await res.json().catch(() => ({}));
      alert(errBody.error || "Gagal menghapus produk (mungkin sudah berubah, muat ulang dulu)");
      loadProducts();
      return;
    }

    handleAsyncResponse(res, `Hapus "${product.name}"`);
  }
});

document.getElementById("new-product-btn").addEventListener("click", () => openProductForm(null));
document.getElementById("product-form-cancel").addEventListener("click", closeProductForm);

function openProductForm(product) {
  document.getElementById("product-form-title").textContent =
    product ? "Ubah Produk" : "Produk Baru";
  document.getElementById("product-id").value = product ? product.id : "";
  document.getElementById("product-version").value = product ? product.version : "";
  document.getElementById("product-sku").value = product ? product.sku : "";
  document.getElementById("product-name").value = product ? product.name : "";
  document.getElementById("product-category").value = product ? product.category : "";
  document.getElementById("product-brand").value = product ? product.brand : "";
  document.getElementById("product-price").value = product ? product.price : "";
  hide(document.getElementById("product-form-error"));
  show(document.getElementById("product-form-backdrop"));
}

function closeProductForm() {
  hide(document.getElementById("product-form-backdrop"));
}

document.getElementById("product-form").addEventListener("submit", async (e) => {
  e.preventDefault();

  const id = document.getElementById("product-id").value;
  const version = document.getElementById("product-version").value;

  const payload = {
    sku: document.getElementById("product-sku").value.trim(),
    name: document.getElementById("product-name").value.trim(),
    category: document.getElementById("product-category").value.trim(),
    brand: document.getElementById("product-brand").value.trim(),
    price: document.getElementById("product-price").value.trim(),
  };

  let res;
  if (id) {
    // Update membutuhkan version terbaru yang diketahui client di body.
    payload.version = Number(version);
    res = await api(`/api/products/${id}`, {
      method: "PUT",
      body: JSON.stringify(payload),
    });
  } else {
    res = await api("/api/products", {
      method: "POST",
      body: JSON.stringify(payload),
    });
  }

  if (!res) {
    const errorEl = document.getElementById("product-form-error");
    errorEl.textContent = "Tidak dapat terhubung ke server, coba lagi";
    show(errorEl);
    return;
  }
  if (res.status === 401) return; // sudah dialihkan ke login

  if (res.status >= 400) {
    const errBody = await res.json().catch(() => ({}));
    const errorEl = document.getElementById("product-form-error");
    errorEl.textContent = errBody.error || "Gagal menyimpan produk";
    show(errorEl);
    return;
  }

  closeProductForm();
  handleAsyncResponse(
    res,
    id ? `Ubah "${payload.name}"` : `Produk baru "${payload.name}"`,
    { resetToFirstPageOnSuccess: !id }
  );
});

// ---------- Status pemrosesan (async via RabbitMQ) ----------

async function handleAsyncResponse(res, label, options = {}) {
  const body = await res.json().catch(() => ({}));
  if (!body.operation_id) {
    loadProducts();
    return;
  }
  trackOperation(body.operation_id, label, options);
}

function trackOperation(id, label, options = {}) {
  const entry = {
    id,
    label,
    status: "pending",
    resetToFirstPageOnSuccess: options.resetToFirstPageOnSuccess || false,
  };
  state.operations.unshift(entry);
  renderOperations();
  pollOperation(entry);
}

// Polling cepat (1 detik) untuk beberapa percobaan pertama, lalu melambat jadi
// tiap 3 detik agar tidak membebani server kalau worker sedang lambat memproses.
const OPERATION_POLL_FAST_ATTEMPTS = 10;
const OPERATION_POLL_MAX_ATTEMPTS = 30; // total: 10x1 detik + 20x3 detik ≈ 70 detik

function nextPollDelay(attempt) {
  return attempt < OPERATION_POLL_FAST_ATTEMPTS ? 1000 : 3000;
}

async function pollOperation(entry, attempt = 0) {
  console.log(`[debug] poll operasi ${entry.id}, percobaan ke-${attempt}`);
  const res = await api(`/api/operations/${entry.id}`);

  if (res && res.status === 401) return; // sesi habis, sudah dialihkan ke login

  if (res && res.status === 200) {
    const body = await res.json();
    const op = body.data ?? body; // API membungkus payload dalam `data`, sama seperti /api/products
    console.log(`[debug] respons operasi ${entry.id}:`, op);
    entry.status = op.status;
    entry.errorMessage = op.error_message;
    renderOperations();

    // Backend hanya mengenal 3 status: pending, succeeded, failed.
    if (op.status === "succeeded" || op.status === "failed") {
      const targetOffset =
        op.status === "succeeded" && entry.resetToFirstPageOnSuccess
          ? 0
          : state.pagination.offset;

      console.log(`[debug] operasi ${entry.id} selesai (${op.status}), memanggil loadProducts(${targetOffset})`);
      loadProducts(targetOffset).catch((err) => {
        console.error("Gagal menyegarkan daftar produk setelah operasi selesai:", err);
        showNetworkError();
      });
      return;
    }
  } else {
    console.log(`[debug] respons operasi ${entry.id} bukan 200:`, res && res.status);
  }

  if (attempt < OPERATION_POLL_MAX_ATTEMPTS) {
    setTimeout(() => pollOperation(entry, attempt + 1), nextPollDelay(attempt));
    return;
  }

  // Worker mungkin sedang lambat/penuh antrean: berhenti polling otomatis
  // supaya tidak membebani server terus-menerus, tapi jangan diam-diam saja —
  // beri tahu user dan sediakan cara cek ulang manual tanpa reload halaman.
  entry.timedOut = true;
  renderOperations();
}

function renderOperations() {
  const list = document.getElementById("operation-list");
  list.innerHTML = "";

  for (const op of state.operations) {
    const li = document.createElement("li");
    const detail = op.status === "failed" && op.errorMessage
      ? ` <span class="muted">— ${escapeHTML(op.errorMessage)}</span>`
      : op.timedOut
        ? ` <span class="muted">— worker belum merespons, coba cek lagi</span>`
        : "";
    const retryBtn = op.timedOut
      ? ` <button type="button" class="btn-secondary" data-retry-operation="${escapeHTML(String(op.id))}">Cek lagi</button>`
      : "";

    li.innerHTML =
      `<span>${escapeHTML(op.label)} <span class="muted">#${escapeHTML(String(op.id))}</span>${detail}</span>` +
      `<span class="status-pill status-${escapeHTML(op.status)}">${escapeHTML(op.status)}</span>${retryBtn}`;
    list.appendChild(li);
  }
}

document.getElementById("operation-list").addEventListener("click", (e) => {
  const btn = e.target.closest("button[data-retry-operation]");
  if (!btn) return;

  const id = btn.dataset.retryOperation;
  const entry = state.operations.find((op) => String(op.id) === id);
  if (!entry) return;

  entry.timedOut = false;
  renderOperations();
  pollOperation(entry, 0);
});

// ---------- Init ----------

checkSession();
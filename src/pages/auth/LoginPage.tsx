import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import i18n from "@/i18n";
import axios from "axios";
import LanguageSwitcher from "@/components/LanguageSwitcher";
import { useNavigate } from "react-router-dom";

export default function LoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const [tenant, setTenant] = useState<string>(() => localStorage.getItem("tenant") || "default");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [rememberTenant, setRememberTenant] = useState(true);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string>("");

  const headers = useMemo(
    () => ({
      "X-Tenant-ID": tenant || "default",
      "Accept-Language": i18n.language?.startsWith("ja") ? "ja" : "en",
    }),
    [tenant]
  );

  useEffect(() => {
    if (rememberTenant) localStorage.setItem("tenant", tenant || "default");
  }, [tenant, rememberTenant]);

  const submit = async (e?: React.FormEvent) => {
    e?.preventDefault();
    setErr("");
    if (!tenant) return setErr(t("Missing tenant"));
    if (!username || !password) return setErr(t("Required"));

    setLoading(true);
    try {
      const res = await axios.post(
        "/v1/auth/login",
        { username, password },
        { headers }
      );

      const token = res.data?.token || res.data?.accessToken || res.data?.jwt;
      if (token) localStorage.setItem("token", token);

      axios.defaults.headers.common["Authorization"] = `Bearer ${token}`;
      axios.defaults.headers.common["X-Tenant-ID"] = tenant;
      axios.defaults.headers.common["Accept-Language"] = headers["Accept-Language"];

      navigate("/databases");
    } catch (e: any) {
      const msg = e?.response?.status === 401 ? t("Invalid credentials") : e?.response?.data?.detail || e?.message;
      setErr(msg || t("Invalid credentials"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-muted/10">
      <form onSubmit={submit} className="w-[380px] bg-white shadow p-6 rounded-xl space-y-4">
        <div className="flex justify-end"><LanguageSwitcher /></div>
        <h1 className="text-2xl font-semibold">{t("Sign in")}</h1>

        {err && (
          <div role="alert" className="text-sm text-red-600 bg-red-50 border border-red-200 rounded px-3 py-2">
            {err}
          </div>
        )}

        <label className="block">
          <span className="block text-sm mb-1">{t("Tenant")}</span>
          <input
            className="w-full border rounded px-3 py-2"
            value={tenant}
            onChange={(e) => setTenant(e.target.value)}
            placeholder="default"
            autoComplete="organization"
            aria-label={t("Tenant")}
          />
        </label>

        <label className="block">
          <span className="block text-sm mb-1">{t("Username")}</span>
          <input
            className="w-full border rounded px-3 py-2"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder={t("Username")}
            autoComplete="username"
            aria-label={t("Username")}
          />
        </label>

        <label className="block">
          <span className="block text-sm mb-1">{t("Password")}</span>
          <input
            className="w-full border rounded px-3 py-2"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={t("Password")}
            autoComplete="current-password"
            aria-label={t("Password")}
            onKeyDown={(e) => e.key === "Enter" && submit()}
          />
        </label>

        <label className="inline-flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={rememberTenant}
            onChange={(e) => setRememberTenant(e.target.checked)}
          />
          {t("Remember tenant")}
        </label>

        <button
          type="submit"
          disabled={loading}
          className="w-full rounded-lg px-4 py-2 bg-blue-600 text-white disabled:opacity-60"
          aria-busy={loading}
        >
          {loading ? t("Logging in...") : t("Login")}
        </button>
      </form>
    </div>
  );
}

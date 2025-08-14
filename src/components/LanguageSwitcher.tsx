import { useTranslation } from "react-i18next";

export default function LanguageSwitcher() {
  const { i18n } = useTranslation();

  const toggle = () => {
    const next = i18n.language?.startsWith("ja") ? "en" : "ja";
    i18n.changeLanguage(next);
  };

  return (
    <button type="button" onClick={toggle} className="text-sm underline">
      {i18n.language?.startsWith("ja") ? "EN" : "日本語"}
    </button>
  );
}

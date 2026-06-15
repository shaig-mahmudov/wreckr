"use client";

import { useEffect, useState } from "react";
import { defaultAPIURL } from "./scenario-api";

const storageKey = "wreckr.apiURL";

export function useAPIURL() {
  const [apiURL, setAPIURLState] = useState(defaultAPIURL);

  useEffect(() => {
    const stored = window.localStorage.getItem(storageKey);
    if (stored) {
      setAPIURLState(stored);
    }
  }, []);

  function setAPIURL(value: string) {
    setAPIURLState(value);
    window.localStorage.setItem(storageKey, value);
  }

  return [apiURL, setAPIURL] as const;
}

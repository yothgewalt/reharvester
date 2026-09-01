"use client";
import { createTheme } from "@mui/material/styles";

import { color, motion } from "./tokens";



const heading = (px: number) => ({
  fontFamily: "var(--font-display)",
  fontWeight: 430,
  fontSize: `${px / 16}rem`,
  lineHeight: 1.3,
  letterSpacing: "-0.03em",
});

export const theme = createTheme({
  cssVariables: { colorSchemeSelector: "data-mui-color-scheme" }, // → [data-mui-color-scheme="dark"]
  colorSchemes: {
    light: {
      palette: {
        primary: { main: color.slab, contrastText: color.white }, // CTA = near-black
        text: { primary: color.ink, secondary: color.ink2, disabled: color.ink4 },
        background: { default: color.white, paper: color.white },
        divider: color.line,

        action: { disabled: color.ink3, disabledBackground: color.line },
      },
    },
    // NOTE: deliberately NO dark scheme. The app is light-only (plane.so taste);
    // dark surfaces (canvas, terminal) are Tailwind colors, not MUI schemes.
    // Defining `dark` here made MUI follow the OS dark mode ("system" default) and
    // paint light-gray text onto the white Tailwind surfaces.
  },
  shape: { borderRadius: 8 },
  typography: {
    fontFamily: "var(--font-inter)",
    h1: heading(58),
    h2: heading(44),
    h3: heading(40),
    h4: heading(32),
    h5: heading(24),
    h6: heading(20),
    body1: { fontSize: "1rem", lineHeight: 1.5 },
    body2: { fontSize: "0.875rem", lineHeight: 1.5 },
    subtitle1: { fontSize: "0.875rem", fontWeight: 500 }, // 14px UI
    button: { fontSize: "0.875rem", fontWeight: 600, textTransform: "none" as const },
    caption: { fontSize: "0.8125rem", lineHeight: 1.45 }, // 13px UI
    overline: {

      fontFamily: "var(--font-plex-mono)",
      fontSize: "0.8125rem",
      fontWeight: 500,
      textTransform: "uppercase" as const,
      letterSpacing: "0.08em",
      lineHeight: 1.5,
    },
  },
  transitions: {
    easing: {
      easeInOut: motion.easing,
      easeOut: motion.easing,
      easeIn: motion.easing,
      sharp: motion.easing,
    },
    duration: {
      shortest: 150,
      shorter: 150,
      short: 200,
      standard: 200,
      complex: 300,
      enteringScreen: 200,
      leavingScreen: 150,
    },
  },
  components: {
    MuiButtonBase: { defaultProps: { disableRipple: true } },
    MuiButton: {
      defaultProps: { disableElevation: true },
      styleOverrides: {
        root: {
          borderRadius: 6,
          padding: "8px 12px",
          transition: `background-color 150ms ${motion.easing}, color 150ms ${motion.easing}, border-color 150ms ${motion.easing}`,
        },
        contained: ({ theme }) => ({
          "&:hover": { backgroundColor: color.ink }, // #0F0F10 → #1D1F20
          ...theme.applyStyles("dark", { "&:hover": { backgroundColor: color.white } }),
        }),
        outlined: ({ theme }) => ({
          borderColor: theme.vars.palette.divider,
          color: theme.vars.palette.text.primary,
          "&:hover": {
            borderColor: theme.vars.palette.text.disabled,
            backgroundColor: "transparent",
          },
        }),
      },
    },
    MuiOutlinedInput: {
      styleOverrides: {
        root: ({ theme }) => ({
          borderRadius: 8,
          "& .MuiOutlinedInput-notchedOutline": { borderColor: theme.vars.palette.divider },
          "&:hover .MuiOutlinedInput-notchedOutline": {
            borderColor: theme.vars.palette.text.disabled,
          },
          "&.Mui-focused .MuiOutlinedInput-notchedOutline": {
            borderColor: theme.vars.palette.text.primary, // ink focus, never blue
            borderWidth: 1,
          },

          "& .MuiOutlinedInput-input::placeholder": { color: color.ink3, opacity: 1 },

          "&.Mui-disabled .MuiOutlinedInput-input": { WebkitTextFillColor: color.ink2 },
        }),
        input: { fontSize: "0.875rem" },
      },
    },
    MuiFormHelperText: {
      styleOverrides: {
        root: { "&.Mui-disabled": { color: color.ink3 } }, // stays readable mid-crawl
      },
    },
    MuiPaper: {
      defaultProps: { elevation: 0 },
      styleOverrides: { root: { backgroundImage: "none" } },
    },
    MuiCard: {
      styleOverrides: {
        root: ({ theme }) => ({
          borderRadius: 12,
          boxShadow: `0 0 0 1px ${theme.vars.palette.divider}`, // ring, never blur
        }),
      },
    },
    MuiLinearProgress: {
      styleOverrides: {
        root: ({ theme }) => ({
          height: 4,
          borderRadius: 2,
          backgroundColor: theme.vars.palette.divider,
        }),
        bar: { borderRadius: 2 }, // inherits primary = near-black
      },
    },
    MuiSlider: {
      styleOverrides: {
        root: { height: 2 },
        thumb: ({ theme }) => ({
          width: 14,
          height: 14,
          boxShadow: `0 0 0 1px ${theme.vars.palette.divider}`,
          "&:hover, &.Mui-focusVisible": {
            boxShadow: `0 0 0 1px ${theme.vars.palette.text.primary}`,
          },
          "&::after": { width: 32, height: 32 },
        }),
        valueLabel: {
          fontFamily: "var(--font-plex-mono)",
          fontSize: "0.8125rem",
          borderRadius: 6,
          backgroundColor: color.slab,
          padding: "2px 8px",
        },
        mark: { display: "none" },
        markLabel: ({ theme }) => ({
          fontSize: "0.8125rem",
          fontFamily: "var(--font-plex-mono)",
          color: theme.vars.palette.text.secondary,
        }),
      },
    },
    MuiSwitch: { defaultProps: { size: "small" } },
    MuiChip: {
      styleOverrides: {
        root: { borderRadius: 6, fontSize: "0.8125rem", fontWeight: 500, height: 26 },
        outlined: ({ theme }) => ({ borderColor: theme.vars.palette.divider }),
      },
    },
    MuiToggleButton: {
      styleOverrides: {
        root: ({ theme }) => ({
          borderRadius: 6,
          padding: "6px 12px",
          fontSize: "0.8125rem",
          fontWeight: 500,
          textTransform: "none",
          borderColor: theme.vars.palette.divider,
          color: theme.vars.palette.text.secondary,
          "&.Mui-selected": {
            color: theme.vars.palette.text.primary,
            backgroundColor: theme.vars.palette.action.selected,
          },
        }),
      },
    },
    MuiMenu: {
      styleOverrides: {
        paper: ({ theme }) => ({
          borderRadius: 8,
          boxShadow: `0 0 0 1px ${theme.vars.palette.divider}`, // ring, never blur
        }),
      },
    },
    MuiAlert: {
      styleOverrides: {

        root: {
          borderRadius: 8,
          "&.MuiAlert-standardWarning": {
            backgroundColor: color.warnBg,
            color: color.warn,
            "& .MuiAlert-icon": { color: color.warn },
          },
          "&.MuiAlert-standardError": {
            backgroundColor: "#FBEDEC",
            color: color.error,
            "& .MuiAlert-icon": { color: color.error },
          },
        },
      },
    },
    MuiBackdrop: {
      styleOverrides: { root: { backgroundColor: "rgba(15, 15, 16, 0.6)" } },
    },
    MuiTooltip: {
      styleOverrides: {
        tooltip: {
          backgroundColor: color.slab,
          borderRadius: 6,
          fontSize: "0.8125rem",
          padding: "6px 10px",
        },
      },
    },
  },
});

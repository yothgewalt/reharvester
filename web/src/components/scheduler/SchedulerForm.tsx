"use client";

import Button from "@mui/material/Button";
import FormControl from "@mui/material/FormControl";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
import TextField from "@mui/material/TextField";
import { useMemo, useState, type FormEvent } from "react";

import { useAppStore } from "@/store";

type Frequency = "daily" | "weekly" | "monthly";

const WEEKDAYS = [
  { value: 0, label: "Sunday" },
  { value: 1, label: "Monday" },
  { value: 2, label: "Tuesday" },
  { value: 3, label: "Wednesday" },
  { value: 4, label: "Thursday" },
  { value: 5, label: "Friday" },
  { value: 6, label: "Saturday" },
];

const HOURS = Array.from({ length: 24 }, (_, i) => i);
const MINUTES = Array.from({ length: 60 }, (_, i) => i);
const MONTH_DAYS = Array.from({ length: 31 }, (_, i) => i + 1);

const pad2 = (n: number) => n.toString().padStart(2, "0");


function buildCron(frequency: Frequency, hour: number, minute: number, weekday: number, monthDay: number) {
  const time = `${minute} ${hour}`;
  if (frequency === "daily") return `${time} * * *`;
  if (frequency === "weekly") return `${time} * * ${weekday}`;
  return `${time} ${monthDay} * *`;
}


function describeCron(frequency: Frequency, hour: number, minute: number, weekday: number, monthDay: number) {
  const time = `${pad2(hour)}:${pad2(minute)}`;
  if (frequency === "daily") return `Every day at ${time}`;
  if (frequency === "weekly") return `Every ${WEEKDAYS.find((d) => d.value === weekday)?.label} at ${time}`;
  return `On the ${monthDay}${ordinalSuffix(monthDay)} of every month at ${time}`;
}

function ordinalSuffix(n: number) {
  if (n > 3 && n < 21) return "th";
  switch (n % 10) {
    case 1:
      return "st";
    case 2:
      return "nd";
    case 3:
      return "rd";
    default:
      return "th";
  }
}

export function SchedulerForm() {
  const [name, setName] = useState("");
  const [keywordsRaw, setKeywordsRaw] = useState("");
  const [frequency, setFrequency] = useState<Frequency>("daily");
  const [hour, setHour] = useState(6);
  const [minute, setMinute] = useState(0);
  const [weekday, setWeekday] = useState(1);
  const [monthDay, setMonthDay] = useState(1);
  const schedulerSubmitting = useAppStore((s) => s.schedulerSubmitting);
  const schedulerError = useAppStore((s) => s.schedulerError);
  const registerProfile = useAppStore((s) => s.registerProfile);

  const cron = useMemo(
    () => buildCron(frequency, hour, minute, weekday, monthDay),
    [frequency, hour, minute, weekday, monthDay],
  );
  const cronDescription = useMemo(
    () => describeCron(frequency, hour, minute, weekday, monthDay),
    [frequency, hour, minute, weekday, monthDay],
  );

  const keywords = useMemo(
    () => keywordsRaw.split(",").map((k) => k.trim()).filter((k) => k.length > 0),
    [keywordsRaw],
  );
  const canSubmit = name.trim().length > 0 && keywords.length > 0 && !schedulerSubmitting;

  const reset = () => {
    setName("");
    setKeywordsRaw("");
    setFrequency("daily");
    setHour(6);
    setMinute(0);
    setWeekday(1);
    setMonthDay(1);
  };

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!canSubmit) return;
    const ok = await registerProfile({ name: name.trim(), keywords, cron });
    if (!ok) return;
    reset();
  };

  return (
    <form onSubmit={handleSubmit} className="flex flex-wrap items-end gap-3">
      <TextField
        size="small"
        label="Rule name"
        value={name}
        onChange={(e) => setName(e.target.value)}
      />
      <TextField
        size="small"
        label="Keywords (comma-separated)"
        value={keywordsRaw}
        onChange={(e) => setKeywordsRaw(e.target.value)}
        className="min-w-64"
      />
      <FormControl size="small" className="min-w-32">
        <InputLabel id="scheduler-frequency-label">Cadence</InputLabel>
        <Select
          labelId="scheduler-frequency-label"
          label="Cadence"
          value={frequency}
          onChange={(e) => setFrequency(e.target.value as Frequency)}
        >
          <MenuItem value="daily">Daily</MenuItem>
          <MenuItem value="weekly">Weekly</MenuItem>
          <MenuItem value="monthly">Monthly</MenuItem>
        </Select>
      </FormControl>

      <FormControl size="small" className="min-w-20">
        <InputLabel id="scheduler-hour-label">Hour</InputLabel>
        <Select
          labelId="scheduler-hour-label"
          label="Hour"
          value={hour}
          onChange={(e) => setHour(Number(e.target.value))}
        >
          {HOURS.map((h) => (
            <MenuItem key={h} value={h}>
              {pad2(h)}
            </MenuItem>
          ))}
        </Select>
      </FormControl>

      <FormControl size="small" className="min-w-20">
        <InputLabel id="scheduler-minute-label">Minute</InputLabel>
        <Select
          labelId="scheduler-minute-label"
          label="Minute"
          value={minute}
          onChange={(e) => setMinute(Number(e.target.value))}
        >
          {MINUTES.map((m) => (
            <MenuItem key={m} value={m}>
              {pad2(m)}
            </MenuItem>
          ))}
        </Select>
      </FormControl>

      {frequency === "weekly" && (
        <FormControl size="small" className="min-w-32">
          <InputLabel id="scheduler-weekday-label">Day</InputLabel>
          <Select
            labelId="scheduler-weekday-label"
            label="Day"
            value={weekday}
            onChange={(e) => setWeekday(Number(e.target.value))}
          >
            {WEEKDAYS.map((d) => (
              <MenuItem key={d.value} value={d.value}>
                {d.label}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      )}

      {frequency === "monthly" && (
        <FormControl size="small" className="min-w-24">
          <InputLabel id="scheduler-month-day-label">Day</InputLabel>
          <Select
            labelId="scheduler-month-day-label"
            label="Day"
            value={monthDay}
            onChange={(e) => setMonthDay(Number(e.target.value))}
          >
            {MONTH_DAYS.map((d) => (
              <MenuItem key={d} value={d}>
                {d}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      )}

      <Button type="submit" variant="outlined" disabled={!canSubmit}>
        Register crawler
      </Button>

      <span className="basis-full text-[13px] text-ink-2 font-mono">
        {cronDescription} — cron: {cron}
      </span>

      {schedulerError ? (
        <span role="alert" className="basis-full text-[13px] text-error">
          {schedulerError} — fields kept, try again.
        </span>
      ) : null}
    </form>
  );
}

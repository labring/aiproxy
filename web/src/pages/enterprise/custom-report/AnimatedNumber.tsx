import { useEffect, useRef, useState } from "react"
import { animate } from "motion"

interface AnimatedNumberProps {
    value: number
    format: (n: number) => string
    duration?: number
}

export function AnimatedNumber({ value, format, duration = 0.6 }: AnimatedNumberProps) {
    const [display, setDisplay] = useState(format(0))
    const prevRef = useRef(0)

    useEffect(() => {
        const from = prevRef.current
        const to = value
        prevRef.current = to

        if (from === to) {
            setDisplay(format(to))
            return
        }

        const controls = animate(from, to, {
            duration,
            ease: "easeOut",
            onUpdate: (latest) => setDisplay(format(latest)),
        })

        return () => controls.stop()
    }, [value, format, duration])

    return <span>{display}</span>
}

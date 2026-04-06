# Användarguide

Välkommen till ms-utrustning — bokningssystemet för Mälarscouternas utrustning. Här kan du bläddra bland tält, kök, verktyg och annan utrustning, boka det du behöver, och hantera utlämning och återlämning.

---

## Innehåll

- [Del 1: För dig som bokar](#del-1-för-dig-som-bokar)
  - [Bläddra utrustning](#bläddra-utrustning)
  - [Boka utrustning](#boka-utrustning)
  - [Hur tilldelning fungerar](#hur-tilldelning-fungerar)
  - [Godkännande](#godkännande)
  - [Hämta ut utrustning](#hämta-ut-utrustning)
  - [Lämna tillbaka](#lämna-tillbaka)
  - [Rapportera problem](#rapportera-problem)
- [Del 2: För utrustningsansvariga](#del-2-för-utrustningsansvariga)
  - [Godkänna bokningar](#godkänna-bokningar)
  - [Hantera ärenden](#hantera-ärenden)
  - [Artikelstatus](#artikelstatus)

---

## Del 1: För dig som bokar

### Bläddra utrustning

Under **Utrustning** ser du allt som finns att boka. Artiklarna är grupperade efter produkttyp och förråd — till exempel "Stormkök" i Hajkförrådet visas som en grupp, och "Stormkök" i Östergården som en annan.

Varje grupp visar hur många som finns tillgängliga just nu, till exempel **3/5 st**. Klicka på en grupp för att se de enskilda artiklarna, var de står, och vilken status de har.

Du kan filtrera på kategori, förråd och fritext. Vill du se arkiverade artiklar finns en kryssruta för det.

**Två typer av artiklar:**

- **Individuellt spårade** — varje exemplar har ett eget namn (t.ex. "Primus 1", "Primus 2"). Vid utlämning ser du exakt vilken du ska hämta.
- **Antalsspårade** — saker där det inte spelar roll vilken du tar (t.ex. liggunderlag, hajkbrickor). Du bokar ett antal, och systemet håller koll på att det räcker.

### Boka utrustning

1. Gå till **Boka** i menyn.
2. Välj start- och slutdatum för din bokning.
3. Välj vem bokningen gäller — din avdelning, ett projekt, eller en personlig bokning.
4. Klicka **Lägg till utrustning** för att se vad som är ledigt under dina datum.
5. Lägg till det du behöver. Du väljer produkttyp och antal — systemet tilldelar specifika artiklar åt dig.
6. När du är nöjd, klicka **Skicka bokning**.

**Bra att veta:**

- Bokningen förväljer din första avdelning. Du kan ändra till en annan avdelning, ett projekt, eller personlig bokning — även efter att bokningen är skapad.
- Du kan bara boka för avdelningar och projekt du tillhör. Utrustningsansvariga kan boka för alla.
- Datum går inte att ändra efter att du lagt till artiklar — avbryt och börja om om du behöver andra datum.
- Du kan lägga till en anteckning (t.ex. "Hajk med Yggdrasil") så det blir lättare att hitta bokningen sen.
- Samma produkttyp i olika förråd visas separat i tillgänglighetslistan. Tänk på att välja rätt förråd om det spelar roll var du hämtar.

### Hur tilldelning fungerar

När du bokar väljer du produkttyp och antal — till exempel "3 Stormkök". Du väljer alltså inte *vilka* stormkök du får. Systemet tilldelar specifika artiklar åt dig baserat på vad som är ledigt under dina datum.

Tilldelningen prioriterar i den här ordningen:

1. **OK** — artiklar utan anmärkningar väljs först.
2. **Inkommande** — beställda artiklar som har ett förväntat leveransdatum *före* din boknings startdatum. De räknas som tillgängliga, men bara om de beräknas vara på plats i tid.
3. **Under reparation** — artiklar som har ett förväntat klardatum *före* din boknings startdatum. Samma logik som inkommande.
4. **Felrapporterad — användbar** — artiklar med ett rapporterat problem som fortfarande går att använda. De tilldelas sist, så du får dem bara om det inte finns bättre alternativ.

Artiklar som är **felrapporterade — ej användbara**, **saknas**, eller **arkiverade** tilldelas aldrig.

Det här betyder att om det finns 6 stormkök i Hajkförrådet men 1 är under reparation och 1 har en felrapport, så ser du "4/6 st" som tillgängliga. Bokar du 4 får du de 4 som är OK. Bokar du 5 får du 4 OK + 1 felrapporterad (användbar). Bokar du 6 går det inte — den under reparation räknas bara om den beräknas vara klar innan din bokning börjar.

Vid utlämning ser du exakt vilka artiklar du tilldelats. Om något inte stämmer kan du byta till en annan tillgänglig artikel av samma typ.

### Godkännande

Vissa artiklar kräver godkännande från utrustningsansvarig innan bokningen bekräftas. Det finns tre nivåer:

- **Fritt bokbar** — bokningen bekräftas direkt.
- **Låg nivå** — projektledare får automatiskt godkännande. Vanliga ledare behöver godkännande från utrustningsansvarig.
- **Hög nivå** — alla utom utrustningsansvariga behöver godkännande.

Om *någon* artikel i din bokning kräver godkännande, väntar hela bokningen. Du ser statusen på bokningssidan.

Om bokningen nekas får du ett meddelande med anledningen. Bokningen går tillbaka till utkast så du kan ändra och skicka igen.

**Vill du ha bekräftelse ändå?** Även om alla artiklar är fritt bokbara kan du kryssa i "Vill ha bekräftelse från ansvarig" när du skickar. Då går bokningen till godkännandekön istället för att bekräftas direkt.

Du kan skriva ett meddelande till utrustningsansvarig när du skickar bokningen, och lägga till kommentarer i bokningens konversationstråd.

### Hämta ut utrustning

När din bokning är bekräftad (eller godkänd) kan du starta utlämningen:

1. Öppna bokningen och klicka **Starta utlämning**.
2. Du ser en checklista med alla artiklar — vilka specifika saker du ska hämta och var de finns (t.ex. "Stormkök 10 — hylla 3 i Hajkförrådet").
3. Bocka av varje artikel när du hämtar den.
4. Om en artikel inte finns där den ska, eller om du vill byta till en annan — använd **byt**-funktionen för att välja en annan tillgänglig artikel av samma typ.

Alla som tillhör samma avdelning kan se och hantera bokningen — det behöver inte vara samma person som skapade den. Det gäller hela flödet: se bokningsdetaljer, följa godkännandekonversationen, hämta ut, och lämna tillbaka. Det går till och med att vara inne i bokningen flera samtidigt, ifall ni står tillsammans för att hämta ut saker eller har delat upp er på olika förråd.

### Lämna tillbaka

När det är dags att lämna tillbaka:

1. Öppna bokningen och klicka **Starta återlämning**.
2. För varje artikel, välj status:
   - **OK** — allt i sin ordning.
   - **Försenad** — du har inte artikeln med dig just nu, men den kommer. Bokningen hålls öppen.
   - **Trasig** — något är sönder. Ett ärende skapas automatiskt.
   - **Saknas** — artikeln är borta. Ett ärende skapas och artikeln markeras som saknad.
3. Du behöver inte lämna tillbaka allt på en gång. Delåterlämningar fungerar — en ledare lämnar tre saker på söndag, en annan lämnar resten på måndag.

Bokningen stängs automatiskt när alla artiklar är återlämnade.

### Rapportera problem

Ser du att något är trasigt eller saknas? Du kan rapportera problem på vilken artikel som helst, när som helst — även om den är utlånad till någon annan.

1. Gå till **Utrustning**, hitta artikeln och klicka **Rapportera**.
2. Beskriv problemet och välj om artikeln fortfarande är användbar eller inte.

Utrustningsansvariga ser alla rapporterade problem under **Ärenden**.

---

## Del 2: För utrustningsansvariga

Som utrustningsansvarig har du tillgång till allt ovan, plus extra funktioner för att hantera inventariet och bokningar.

### Godkänna bokningar

Under **Bokningar** ser du en flik för bokningar som väntar på godkännande (med en räknare). Klicka på en bokning för att se detaljerna.

- **Godkänn** — bokningen bekräftas och bokaren meddelas.
- **Neka** — bokningen går tillbaka till utkast. Skriv gärna en kommentar så bokaren vet varför.

Du kan också skriva kommentarer i bokningens konversationstråd utan att godkänna eller neka — till exempel för att ställa en fråga.

### Hantera ärenden

Under **Ärenden** ser du alla artiklar som har en rapporterad status. Du kan filtrera på statustyp och se bara dina egna ärenden.

Klicka på en artikel för att se historiken och ändra status:

- **OK (löst)** — problemet är åtgärdat, artikeln är tillbaka i drift.
- **Under reparation** — artikeln är inskickad eller under lagning. Den blir inte bokbar förrän du sätter ett förväntat tillgänglighetsdatum.
- **Saknas** — artikeln är borttappad.
- **Arkiverad** — artikeln är uttjänt och tas ur inventariet.

Varje statusändring loggas med datum, vem som gjorde det, och en valfri kommentar. Det ger en komplett historik per artikel.

### Artikelstatus

Artiklar har en status som beskriver deras *skick* — det är skilt från om de är bokade eller utlånade (det räknas ut automatiskt från bokningsdata).

| Status | Bokbar? | Beskrivning |
|---|---|---|
| OK | Ja | Fungerar, inga problem |
| Felrapporterad — användbar | Ja | Rapporterat problem, men går att använda |
| Inkommande | Ja (framtida) | Beställd, bokbar för datum efter förväntat leveransdatum |
| Felrapporterad — ej användbar | Nej | Rapporterat problem, går inte att använda |
| Under reparation | Nej (tills datum) | Inskickad, bokbar efter förväntat klardatum |
| Saknas | Nej | Borttappad |
| Arkiverad | Nej | Uttjänt, ur inventariet |

Artiklar med status "Inkommande" eller "Under reparation" kan ha ett förväntat tillgänglighetsdatum. De blir bokbara för perioder som börjar på eller efter det datumet.

---

## På gång

Funktioner som planeras men inte är byggda ännu:

- Paket — färdiga utrustningsset för vanliga scenarier (t.ex. "hajk för 8 utmanare")
- Notifieringar via e-post och Google Chat
- Artikelbilder vid felrapportering
- Inventariehantering i appen (skapa/redigera artiklar, bulk-åtgärder, CSV-import/-export)
- Utskriftsvänlig hämtlista
- Statistik och rapporter (utlåningshistorik per artikel, person, förråd)

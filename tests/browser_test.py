
import asyncio
from playwright.async_api import async_playwright
import time

async def run():
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True) # Set headless=False for visual debugging
        context = await browser.new_context()

        # 1. Setup Mattermost Admin
        print("Setting up Mattermost...")
        page_mm = await context.new_page()
        try:
            await page_mm.goto("http://localhost:8065", timeout=60000)
            # Wait for startup
            await page_mm.wait_for_selector("input[id='input_email']", timeout=60000) 
            
            # Initial setup (if not already done)
            if "signup_email" in page_mm.url:
               await page_mm.fill("input[pluginid='email']", "admin@example.com")
               await page_mm.fill("input[pluginid='username']", "sysadmin")
               await page_mm.fill("input[pluginid='password']", "Sys@dmin123")
               await page_mm.click("button[id='create_account']")
               
               # Create Team
               await page_mm.wait_for_selector("a[id='create_team']")
               await page_mm.click("a[id='create_team']")
               await page_mm.fill("input[id='team_name']", "Test Team")
               await page_mm.click("button[type='submit']")
               await page_mm.fill("input[id='team_url']", "test-team")
               await page_mm.click("button[type='submit']")
               await page_mm.click("button:has-text('Finish')")
               
        except Exception as e:
            print(f"Mattermost setup skipped or failed (might be already setup): {e}")

        # 2. Setup Element/Matrix User
        print("Setting up Element...")
        page_el = await context.new_page()
        await page_el.goto("http://localhost:8080")
        
        # Register
        await page_el.click("a:has-text('Create account')")
        await page_el.click("div[role='button']:has-text('Edit')") # Edit homeserver
        await page_el.fill("input[id='homeserver']", "http://localhost:8008")
        await page_el.click("div[role='button']:has-text('Continue')")

        username = f"user_{int(time.time())}"
        await page_el.fill("input[id='username']", username)
        await page_el.fill("input[id='password']", "password123")
        await page_el.fill("input[id='passwordConfirm']", "password123")
        await page_el.click("div[role='button']:has-text('Register')")
        
        # 3. Start Chat with Bridge Bot (or DM)
        print("Starting chat with Mattermost user...")
        # TODO: This requires knowing the ghost MXID structure or user search
        # Typically @mattermost_sysadmin:localhost 
        
        await page_el.click("div[aria-label='Start chat']")
        await page_el.fill("input[type='text']", "@mattermost_sysadmin:localhost") 
        await page_el.click("div[role='button']:has-text('Go')")
        
        # Send message
        await page_el.fill("div[contenteditable='true']", "Hello from Matrix!")
        await page_el.press("div[contenteditable='true']", "Enter")

        # 4. Verify in Mattermost
        print("Verifying in Mattermost...")
        await page_mm.bring_to_front()
        # Navigate to DM or channel? Double puppeting vs Relay?
        # If relay is enabled, bot should post to channel.
        # But we DMed a specific user.
        
        # For simplicity, let's look for "Hello from Matrix!" text anywhere
        await page_mm.wait_for_selector("div:has-text('Hello from Matrix!')")
        print("SUCCESS: Message received in Mattermost!")
        
        await browser.close()

if __name__ == "__main__":
    asyncio.run(run())
